package database

import (
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/database/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

func normalizeIPv4SharedInboundListeners() error {
	var inbounds []model.Inbound
	if err := db.Select("id", "options", "out_json").Find(&inbounds).Error; err != nil {
		return err
	}
	for _, inbound := range inbounds {
		var options map[string]interface{}
		if err := json.Unmarshal(inbound.Options, &options); err != nil || options["listen"] != "::" {
			continue
		}
		var outbound map[string]interface{}
		if err := json.Unmarshal(inbound.OutJson, &outbound); err != nil {
			continue
		}
		server, _ := outbound["server"].(string)
		ip := net.ParseIP(strings.Trim(server, "[]"))
		if ip == nil || ip.To4() == nil {
			continue
		}
		options["listen"] = "0.0.0.0"
		updated, err := json.MarshalIndent(options, "", "  ")
		if err != nil {
			return err
		}
		if err = db.Model(&model.Inbound{}).Where("id = ?", inbound.Id).UpdateColumn("options", updated).Error; err != nil {
			return err
		}
	}
	return nil
}

func OpenDB(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, 01740)
	if err != nil {
		return err
	}

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	sep := "?"
	if strings.Contains(dbPath, "?") {
		sep = "&"
	}
	// _cache_size=-200 caps each connection's page cache at about 200 KiB,
	// reducing memory amplification if a connection escapes the pool.
	dsn := dbPath + sep + "_busy_timeout=10000&_journal_mode=WAL&_cache_size=-200"
	db, err = gorm.Open(sqlite.Open(dsn), c)
	if err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(maxOpenConnections(runtime.GOMAXPROCS(0)))
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	if config.IsDebug() {
		db = db.Debug()
	}
	return nil
}

func maxOpenConnections(processors int) int {
	if processors <= 1 {
		return 4
	}
	return 8
}

func InitDB(dbPath string) error {
	err := OpenDB(dbPath)
	if err != nil {
		return err
	}

	// Default Outbounds
	if !db.Migrator().HasTable(&model.Outbound{}) {
		db.Migrator().CreateTable(&model.Outbound{})
		defaultOutbound := []model.Outbound{
			{Type: "direct", Tag: "direct", Options: json.RawMessage(`{}`)},
		}
		db.Create(&defaultOutbound)
	}

	if err = dedupStats(); err != nil {
		return err
	}

	err = db.AutoMigrate(
		&model.Setting{},
		&model.Tls{},
		&model.TlsAuthority{},
		&model.Inbound{},
		&model.Outbound{},
		&model.Service{},
		&model.Endpoint{},
		&model.User{},
		&model.Tokens{},
		&model.Stats{},
		&model.Client{},
		&model.Changes{},
		&model.RelayPool{},
		&model.AgentNode{},
		&model.SdwanMember{},
	)
	if err != nil {
		return err
	}
	if err = normalizeIPv4SharedInboundListeners(); err != nil {
		return err
	}
	return migrateLegacySdwanMembers()
}

// migrateLegacySdwanMembers turns members stored by the first SD-WAN version
// (one uplink in the protocol/port/outbound columns) into a single path, so
// they keep working until one-click tuning upgrades them.
func migrateLegacySdwanMembers() error {
	migrator := db.Migrator()
	if !migrator.HasColumn(&model.SdwanMember{}, "outbound") || !migrator.HasColumn(&model.SdwanMember{}, "protocol") {
		return nil
	}
	var rows []struct {
		Id       uint
		Protocol sql.NullString
		Port     sql.NullInt64
		Outbound sql.NullString
		Paths    sql.NullString
	}
	if err := db.Table("sdwan_members").Select("id, protocol, port, outbound, paths").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		paths := strings.TrimSpace(row.Paths.String)
		outbound := strings.TrimSpace(row.Outbound.String)
		if (paths != "" && paths != "null") || outbound == "" || !json.Valid([]byte(outbound)) {
			continue
		}
		converted, err := json.Marshal([]map[string]interface{}{{
			"protocol": row.Protocol.String, "port": row.Port.Int64, "outbound": json.RawMessage(outbound),
		}})
		if err != nil {
			return err
		}
		if err = db.Table("sdwan_members").Where("id = ?", row.Id).Update("paths", string(converted)).Error; err != nil {
			return err
		}
	}
	return nil
}

// dedupStats merges duplicate traffic buckets before AutoMigrate adds the
// unique stats index introduced by newer upstream versions.
func dedupStats() error {
	if !db.Migrator().HasTable(&model.Stats{}) {
		return nil
	}

	var dupGroups int64
	err := db.Raw("SELECT COUNT(*) FROM (SELECT 1 FROM stats GROUP BY resource, tag, date_time, direction HAVING COUNT(*) > 1)").Scan(&dupGroups).Error
	if err != nil {
		return err
	}
	if dupGroups == 0 {
		return nil
	}
	log.Printf("stats: collapsing %d duplicate group(s) before adding unique index", dupGroups)

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TEMP TABLE stats_dedup AS
			SELECT MIN(id) AS id, resource, tag, date_time, direction, SUM(traffic) AS traffic
			FROM stats GROUP BY resource, tag, date_time, direction`).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM stats").Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO stats (id, resource, tag, date_time, direction, traffic)
			SELECT id, resource, tag, date_time, direction, traffic FROM stats_dedup`).Error; err != nil {
			return err
		}
		return tx.Exec("DROP TABLE stats_dedup").Error
	})
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
