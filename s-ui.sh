#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

function LOGD() {
    echo -e "${yellow}[璋冭瘯] $* ${plain}"
}

function LOGE() {
    echo -e "${red}[閿欒] $* ${plain}"
}

function LOGI() {
    echo -e "${green}[淇℃伅] $* ${plain}"
}

[[ $EUID -ne 0 ]] && LOGE "閿欒锛氬繀椤讳娇鐢?root 鏉冮檺杩愯姝よ剼鏈紒\n" && exit 1

if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "妫€娴嬬郴缁熷け璐ワ紝璇疯仈绯讳綔鑰咃紒" >&2
    exit 1
fi

echo "褰撳墠绯荤粺鍙戣鐗堜负锛?release"

confirm() {
    if [[ $# > 1 ]]; then
        echo && read -p "$1 [榛樿$2]: " temp
        if [[ x"${temp}" == x"" ]]; then
            temp=$2
        fi
    else
        read -p "$1 [y/n]锛?" temp
    fi
    if [[ x"${temp}" == x"y" || x"${temp}" == x"Y" ]]; then
        return 0
    else
        return 1
    fi
}

confirm_restart() {
    confirm "閲嶅惎 ${1} 鏈嶅姟" "y"
    if [[ $? == 0 ]]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo && echo -n -e "${yellow}鎸夊洖杞﹁繑鍥炰富鑿滃崟锛?{plain}" && read temp
    show_menu
}

install() {
    bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)
    if [[ $? == 0 ]]; then
        if [[ $# == 0 ]]; then
            start
        else
            start 0
        fi
    fi
}

update() {
    confirm "姝ゅ姛鑳藉皢寮哄埗閲嶈鏈€鏂扮増鏈紝鏁版嵁涓嶄細涓㈠け銆傛槸鍚︾户缁紵" "n"
    if [[ $? != 0 ]]; then
        LOGE "宸插彇娑?
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 0
    fi
    bash <(curl -Ls https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh)
    if [[ $? == 0 ]]; then
        LOGI "鏇存柊瀹屾垚锛岄潰鏉垮凡鑷姩閲嶅惎"
        exit 0
    fi
}

custom_version() {
    echo "璇疯緭鍏ラ潰鏉跨増鏈紙渚嬪 v1.4.1锛夛細"
    read panel_version

    if [ -z "$panel_version" ]; then
        echo "闈㈡澘鐗堟湰涓嶈兘涓虹┖銆傛鍦ㄩ€€鍑恒€?
    exit 1
    fi

    [[ "${panel_version}" != v* ]] && panel_version="v${panel_version}"

    download_link="https://raw.githubusercontent.com/Hhz0823/1s-ui/main/install.sh"

    install_command="bash <(curl -Ls $download_link) $panel_version"

    echo "姝ｅ湪涓嬭浇骞跺畨瑁呴潰鏉跨増鏈?$panel_version..."
    eval $install_command
}

uninstall() {
    confirm "纭畾瑕佸嵏杞介潰鏉垮悧锛? "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi
    systemctl stop s-ui
    systemctl disable s-ui
    rm /etc/systemd/system/s-ui.service -f
    systemctl daemon-reload
    systemctl reset-failed
    rm /etc/s-ui/ -rf
    rm /usr/local/s-ui/ -rf

    echo ""
    echo -e "鍗歌浇鎴愬姛銆傚鏋滆鍒犻櫎姝よ剼鏈紝璇峰湪閫€鍑鸿剼鏈悗杩愯 ${green}rm /usr/local/s-ui -f${plain}銆?
    echo ""

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

reset_admin() {
    echo "涓嶅缓璁皢绠＄悊鍛樿处鍙峰瘑鐮佽缃负榛樿鍊硷紒"
    confirm "纭畾瑕佸皢绠＄悊鍛樿处鍙峰瘑鐮侀噸缃负榛樿鍊煎悧锛? "n"
    if [[ $? == 0 ]]; then
        /usr/local/s-ui/sui admin -reset
    fi
    before_show_menu
}

set_admin() {
    echo "涓嶅缓璁皢绠＄悊鍛樿处鍙峰瘑鐮佽缃负杩囦簬澶嶆潅鐨勬枃鏈€?
    read -p "璇疯缃敤鎴峰悕锛? config_account
    read -p "璇疯缃瘑鐮侊細" config_password
    /usr/local/s-ui/sui admin -username ${config_account} -password ${config_password}
    before_show_menu
}

view_admin() {
    /usr/local/s-ui/sui admin -show
    before_show_menu
}

reset_setting() {
    confirm "纭畾瑕佸皢璁剧疆閲嶇疆涓洪粯璁ゅ€煎悧锛? "n"
    if [[ $? == 0 ]]; then
        /usr/local/s-ui/sui setting -reset
    fi
    before_show_menu
}

set_setting() {
    echo -e "璇疯緭鍏?{yellow}闈㈡澘绔彛${plain}锛堢暀绌哄垯浣跨敤鐜版湁/榛樿鍊硷級锛?
    read config_port
    echo -e "璇疯緭鍏?{yellow}闈㈡澘璺緞${plain}锛堢暀绌哄垯浣跨敤鐜版湁/榛樿鍊硷級锛?
    read config_path

    echo -e "璇疯緭鍏?{yellow}璁㈤槄绔彛${plain}锛堢暀绌哄垯浣跨敤鐜版湁/榛樿鍊硷級锛?
    read config_subPort
    echo -e "璇疯緭鍏?{yellow}璁㈤槄璺緞${plain}锛堢暀绌哄垯浣跨敤鐜版湁/榛樿鍊硷級锛?
    read config_subPath

    echo -e "${yellow}姝ｅ湪鍒濆鍖栵紝璇风◢鍊?..${plain}"
    params=""
    [ -z "$config_port" ] || params="$params -port $config_port"
    [ -z "$config_path" ] || params="$params -path $config_path"
    [ -z "$config_subPort" ] || params="$params -subPort $config_subPort"
    [ -z "$config_subPath" ] || params="$params -subPath $config_subPath"
    /usr/local/s-ui/sui setting ${params}
    before_show_menu
}

view_setting() {
    /usr/local/s-ui/sui setting -show
    view_uri
    before_show_menu
}

view_uri() {
    info=$(/usr/local/s-ui/sui uri)
    if [[ $? != 0 ]]; then
        LOGE "鑾峰彇褰撳墠 URI 澶辫触"
        before_show_menu
    fi
    LOGI "浣犲彲浠ラ€氳繃浠ヤ笅 URL 璁块棶闈㈡澘锛?
    echo -e "${green}${info}${plain}"
}

start() {
    check_status $1
    if [[ $? == 0 ]]; then
        echo ""
        LOGI -e "${1} 姝ｅ湪杩愯锛屾棤闇€鍐嶆鍚姩锛涘鏋滈渶瑕侀噸鍚紝璇烽€夋嫨閲嶅惎"
    else
        systemctl start $1
        sleep 2
        check_status $1
        if [[ $? == 0 ]]; then
            LOGI "${1} 鍚姩鎴愬姛"
        else
            LOGE "鍚姩 ${1} 澶辫触锛屽彲鑳芥槸鍚姩鏃堕棿瓒呰繃涓ょ锛岃绋嶅悗鏌ョ湅鏃ュ織淇℃伅"
        fi
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

stop() {
    check_status $1
    if [[ $? == 1 ]]; then
        echo ""
        LOGI "${1} 宸插仠姝紝鏃犻渶鍐嶆鍋滄锛?
    else
        systemctl stop $1
        sleep 2
        check_status
        if [[ $? == 1 ]]; then
            LOGI "${1} 鍋滄鎴愬姛"
        else
            LOGE "鍋滄 ${1} 澶辫触锛屽彲鑳芥槸鍋滄鏃堕棿瓒呰繃涓ょ锛岃绋嶅悗鏌ョ湅鏃ュ織淇℃伅"
        fi
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

restart() {
    systemctl restart $1
    sleep 2
    check_status $1
    if [[ $? == 0 ]]; then
        LOGI "${1} 閲嶅惎鎴愬姛"
    else
        LOGE "閲嶅惎 ${1} 澶辫触锛屽彲鑳芥槸鍚姩鏃堕棿瓒呰繃涓ょ锛岃绋嶅悗鏌ョ湅鏃ュ織淇℃伅"
    fi
    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

status() {
    systemctl status s-ui -l
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

enable() {
    systemctl enable $1
    if [[ $? == 0 ]]; then
        LOGI "宸叉垚鍔熻缃?${1} 寮€鏈鸿嚜鍚?
    else
        LOGE "璁剧疆 ${1} 寮€鏈鸿嚜鍚け璐?
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

disable() {
    systemctl disable $1
    if [[ $? == 0 ]]; then
        LOGI "宸叉垚鍔熷彇娑?${1} 寮€鏈鸿嚜鍚?
    else
        LOGE "鍙栨秷 ${1} 寮€鏈鸿嚜鍚け璐?
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

show_log() {
    journalctl -u $1.service -e --no-pager -f
    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

update_shell() {
    wget -O /usr/bin/s-ui -N --no-check-certificate https://github.com/Hhz0823/1s-ui/raw/main/s-ui.sh
    if [[ $? != 0 ]]; then
        echo ""
        LOGE "涓嬭浇鑴氭湰澶辫触锛岃妫€鏌ュ綋鍓嶆満鍣ㄦ槸鍚﹀彲浠ヨ繛鎺?Github"
        before_show_menu
    else
        chmod +x /usr/bin/s-ui
        LOGI "鑴氭湰鍗囩骇鎴愬姛锛岃閲嶆柊杩愯鑴氭湰" && exit 0
    fi
}

check_status() {
    if [[ ! -f "/etc/systemd/system/$1.service" ]]; then
        return 2
    fi
    temp=$(systemctl status "$1" | grep Active | awk '{print $3}' | cut -d "(" -f2 | cut -d ")" -f1)
    if [[ x"${temp}" == x"running" ]]; then
        return 0
    else
        return 1
    fi
}

check_enabled() {
    temp=$(systemctl is-enabled $1)
    if [[ x"${temp}" == x"enabled" ]]; then
        return 0
    else
        return 1
    fi
}

check_uninstall() {
    check_status s-ui
    if [[ $? != 2 ]]; then
        echo ""
        LOGE "闈㈡澘宸插畨瑁咃紝璇峰嬁閲嶅瀹夎"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

check_install() {
    check_status s-ui
    if [[ $? == 2 ]]; then
        echo ""
        LOGE "璇峰厛瀹夎闈㈡澘"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

show_status() {
    check_status $1
    case $? in
    0)
        echo -e "${1} 鐘舵€侊細${green}杩愯涓?{plain}"
        show_enable_status $1
        ;;
    1)
        echo -e "${1} 鐘舵€侊細${yellow}鏈繍琛?{plain}"
        show_enable_status $1
        ;;
    2)
        echo -e "${1} 鐘舵€侊細${red}鏈畨瑁?{plain}"
        ;;
    esac
}

show_enable_status() {
    check_enabled $1
    if [[ $? == 0 ]]; then
        echo -e "${1} 寮€鏈鸿嚜鍚細${green}鏄?{plain}"
    else
        echo -e "${1} 寮€鏈鸿嚜鍚細${red}鍚?{plain}"
    fi
}

check_s-ui_status() {
    count=$(ps -ef | grep "sui" | grep -v "grep" | wc -l)
    if [[ count -ne 0 ]]; then
        return 0
    else
        return 1
    fi
}

show_s-ui_status() {
    check_s-ui_status
    if [[ $? == 0 ]]; then
        echo -e "s-ui 鐘舵€侊細${green}杩愯涓?{plain}"
    else
        echo -e "s-ui 鐘舵€侊細${red}鏈繍琛?{plain}"
    fi
}

bbr_menu() {
    echo -e "${green}\t1.${plain} 鍚敤 BBR"
    echo -e "${green}\t2.${plain} 绂佺敤 BBR"
    echo -e "${green}\t0.${plain} 杩斿洖涓昏彍鍗?
    read -p "璇烽€夋嫨涓€涓€夐」锛?" choice
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        enable_bbr
        ;;
    2)
        disable_bbr
        ;;
    *) echo "鏃犳晥閫夋嫨" ;;
    esac
}

disable_bbr() {
    if ! grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf || ! grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${yellow}褰撳墠鏈惎鐢?BBR銆?{plain}"
        exit 0
    fi
    sed -i 's/net.core.default_qdisc=fq/net.core.default_qdisc=pfifo_fast/' /etc/sysctl.conf
    sed -i 's/net.ipv4.tcp_congestion_control=bbr/net.ipv4.tcp_congestion_control=cubic/' /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "cubic" ]]; then
        echo -e "${green}宸叉垚鍔熷皢 BBR 鏇挎崲涓?CUBIC銆?{plain}"
    else
        echo -e "${red}灏?BBR 鏇挎崲涓?CUBIC 澶辫触銆傝妫€鏌ョ郴缁熼厤缃€?{plain}"
    fi
}

enable_bbr() {
    if grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf && grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${green}BBR 宸插惎鐢紒${plain}"
        exit 0
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update && apt-get install -yqq --no-install-recommends ca-certificates
        ;;
    centos | almalinux | rocky | oracle)
        yum -y update && yum -y install ca-certificates
        ;;
    fedora)
        dnf -y update && dnf -y install ca-certificates
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm ca-certificates
        ;;
    *)
        echo -e "${red}涓嶆敮鎸佺殑鎿嶄綔绯荤粺銆傝妫€鏌ヨ剼鏈苟鎵嬪姩瀹夎蹇呰鐨勮蒋浠跺寘銆?{plain}\n"
        exit 1
        ;;
    esac
    echo "net.core.default_qdisc=fq" | tee -a /etc/sysctl.conf
    echo "net.ipv4.tcp_congestion_control=bbr" | tee -a /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "bbr" ]]; then
        echo -e "${green}BBR 鍚敤鎴愬姛銆?{plain}"
    else
        echo -e "${red}鍚敤 BBR 澶辫触銆傝妫€鏌ョ郴缁熼厤缃€?{plain}"
    fi
}

install_acme() {
    cd ~
    LOGI "姝ｅ湪瀹夎 acme..."
    curl https://get.acme.sh | sh
    if [ $? -ne 0 ]; then
        LOGE "瀹夎 acme 澶辫触"
        return 1
    else
        LOGI "瀹夎 acme 鎴愬姛"
    fi
    return 0
}

ssl_cert_issue_main() {
    echo -e "${green}\t1.${plain} 鑾峰彇 SSL"
    echo -e "${green}\t2.${plain} 鍚婇攢璇佷功"
    echo -e "${green}\t3.${plain} 寮哄埗缁"
    echo -e "${green}\t4.${plain} 鑷鍚嶈瘉涔?
    read -p "璇烽€夋嫨涓€涓€夐」锛?" choice
    case "$choice" in
        1) ssl_cert_issue ;;
        2)
            local domain=""
            read -p "璇疯緭鍏ヨ鍚婇攢璇佷功鐨勫煙鍚嶏細 " domain
            ~/.acme.sh/acme.sh --revoke -d ${domain}
            LOGI "璇佷功宸插悐閿€"
            ;;
        3)
            local domain=""
            read -p "璇疯緭鍏ヨ寮哄埗缁 SSL 璇佷功鐨勫煙鍚嶏細 " domain
            ~/.acme.sh/acme.sh --renew -d ${domain} --force ;;
        4)
            generate_self_signed_cert
            ;;
        *) echo "鏃犳晥閫夋嫨" ;;
    esac
}

ssl_cert_issue() {
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        echo "鏈壘鍒?acme.sh锛屽皢杩涜瀹夎"
        install_acme
        if [ $? -ne 0 ]; then
            LOGE "瀹夎 acme 澶辫触锛岃妫€鏌ユ棩蹇?
            exit 1
        fi
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt update && apt install socat -y
        ;;
    centos | almalinux | rocky | oracle)
        yum -y update && yum -y install socat
        ;;
    fedora)
        dnf -y update && dnf -y install socat
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm socat
        ;;
    *)
        echo -e "${red}涓嶆敮鎸佺殑鎿嶄綔绯荤粺銆傝妫€鏌ヨ剼鏈苟鎵嬪姩瀹夎蹇呰鐨勮蒋浠跺寘銆?{plain}\n"
        exit 1
        ;;
    esac
    if [ $? -ne 0 ]; then
        LOGE "瀹夎 socat 澶辫触锛岃妫€鏌ユ棩蹇?
        exit 1
    else
        LOGI "瀹夎 socat 鎴愬姛..."
    fi

    local domain=""
    read -p "璇疯緭鍏ヤ綘鐨勫煙鍚嶏細" domain
    LOGD "浣犵殑鍩熷悕鏄細${domain}锛屾鍦ㄦ鏌?.."
    local currentCert=$(~/.acme.sh/acme.sh --list | tail -1 | awk '{print $1}')

    if [ ${currentCert} == ${domain} ]; then
        local certInfo=$(~/.acme.sh/acme.sh --list)
        LOGE "绯荤粺涓凡瀛樺湪璇佷功锛屼笉鑳介噸澶嶇鍙戯紝褰撳墠璇佷功璇︽儏锛?
        LOGI "$certInfo"
        exit 1
    else
        LOGI "浣犵殑鍩熷悕宸插噯澶囧ソ绛惧彂璇佷功..."
    fi

    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf "$certPath"
        mkdir -p "$certPath"
    fi

    local WebPort=80
    read -p "璇烽€夋嫨浣跨敤鐨勭鍙ｏ紝榛樿浣跨敤 80 绔彛锛? WebPort
    if [[ ${WebPort} -gt 65535 || ${WebPort} -lt 1 ]]; then
        LOGE "杈撳叆鐨?${WebPort} 鏃犳晥锛屽皢浣跨敤榛樿绔彛"
    fi
    LOGI "灏嗕娇鐢ㄧ鍙?${WebPort} 绛惧彂璇佷功锛岃纭繚璇ョ鍙ｅ凡寮€鏀?.."
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
    ~/.acme.sh/acme.sh --issue -d ${domain} --standalone --httpport ${WebPort}
    if [ $? -ne 0 ]; then
        LOGE "绛惧彂璇佷功澶辫触锛岃妫€鏌ユ棩蹇?
        rm -rf ~/.acme.sh/${domain}
        exit 1
    else
        LOGE "璇佷功绛惧彂鎴愬姛锛屾鍦ㄥ畨瑁呰瘉涔?.."
    fi
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem

    if [ $? -ne 0 ]; then
        LOGE "瀹夎璇佷功澶辫触锛岄€€鍑?
        rm -rf ~/.acme.sh/${domain}
        exit 1
    else
        LOGI "瀹夎璇佷功鎴愬姛锛屾鍦ㄥ惎鐢ㄨ嚜鍔ㄧ画绛?.."
    fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        LOGE "鑷姩缁澶辫触锛岃瘉涔﹁鎯咃細"
        ls -lah cert/*
        chmod 755 $certPath/*
        exit 1
    else
        LOGI "鑷姩缁鎴愬姛锛岃瘉涔﹁鎯咃細"
        ls -lah cert/*
        chmod 755 $certPath/*
    fi
}

ssl_cert_issue_CF() {
    echo -E ""
    LOGD "******浣跨敤璇存槑******"
    echo "1) 浠?Cloudflare 鐢宠鏂拌瘉涔?
    echo "2) 寮哄埗缁宸叉湁璇佷功"
    echo "3) 杩斿洖鑿滃崟"
    read -p "璇疯緭鍏ヤ綘鐨勯€夋嫨 [1-3]锛?" choice

    certPath="/root/cert-CF"

    case $choice in
        1|2)
            force_flag=""
            if [ "$choice" -eq 2 ]; then
                force_flag="--force"
                echo "姝ｅ湪寮哄埗閲嶆柊绛惧彂 SSL 璇佷功..."
            else
                echo "寮€濮嬬鍙?SSL 璇佷功..."
            fi

            LOGD "******浣跨敤璇存槑******"
            LOGI "姝?Acme 鑴氭湰闇€瑕佷互涓嬫暟鎹細"
            LOGI "1.Cloudflare 娉ㄥ唽閭"
            LOGI "2.Cloudflare 鍏ㄥ眬 API Key"
            LOGI "3.宸查€氳繃 Cloudflare 灏?DNS 瑙ｆ瀽鍒板綋鍓嶆湇鍔″櫒鐨勫煙鍚?
            LOGI "4.鑴氭湰灏嗙敵璇疯瘉涔︼紝榛樿瀹夎璺緞涓?/root/cert"
            confirm "鏄惁纭锛焄y/n]" "y"
            if [ $? -eq 0 ]; then
                if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
                    echo "鏈壘鍒?acme.sh銆傛鍦ㄥ畨瑁?.."
                    install_acme
                    if [ $? -ne 0 ]; then
                        LOGE "瀹夎 acme 澶辫触锛岃妫€鏌ユ棩蹇?
                        exit 1
                    fi
                fi

                CF_Domain=""
                if [ ! -d "$certPath" ]; then
                    mkdir -p $certPath
                else
                    rm -rf $certPath
                    mkdir -p $certPath
                fi

                LOGD "璇疯缃煙鍚嶏細"
                read -p "璇峰湪姝よ緭鍏ュ煙鍚嶏細 " CF_Domain
                LOGD "浣犵殑鍩熷悕宸茶缃负锛?{CF_Domain}"

                CF_GlobalKey=""
                CF_AccountEmail=""
                LOGD "璇疯缃?API key锛?
                read -p "璇峰湪姝よ緭鍏?key锛?" CF_GlobalKey
                LOGD "浣犵殑 API key 涓猴細${CF_GlobalKey}"

                LOGD "璇疯缃敞鍐岄偖绠憋細"
                read -p "璇峰湪姝よ緭鍏ラ偖绠憋細 " CF_AccountEmail
                LOGD "浣犵殑娉ㄥ唽閭涓猴細${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                if [ $? -ne 0 ]; then
                    LOGE "璁剧疆榛樿 CA Let's Encrypt 澶辫触锛岃剼鏈€€鍑?.."
                    exit 1
                fi

                export CF_Key="${CF_GlobalKey}"
                export CF_Email="${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --issue --dns dns_cf -d ${CF_Domain} -d *.${CF_Domain} $force_flag --log
                if [ $? -ne 0 ]; then
                    LOGE "璇佷功绛惧彂澶辫触锛岃剼鏈€€鍑?.."
                    exit 1
                else
                    LOGI "璇佷功绛惧彂鎴愬姛锛屾鍦ㄥ畨瑁?.."
                fi

                mkdir -p ${certPath}/${CF_Domain}
                if [ $? -ne 0 ]; then
                    LOGE "鍒涘缓鐩綍澶辫触锛?{certPath}/${CF_Domain}"
                    exit 1
                fi

                ~/.acme.sh/acme.sh --installcert -d ${CF_Domain} -d *.${CF_Domain} \
                    --fullchain-file ${certPath}/${CF_Domain}/fullchain.pem \
                    --key-file ${certPath}/${CF_Domain}/privkey.pem

                if [ $? -ne 0 ]; then
                    LOGE "璇佷功瀹夎澶辫触锛岃剼鏈€€鍑?.."
                    exit 1
                else
                    LOGI "璇佷功瀹夎鎴愬姛锛屾鍦ㄥ紑鍚嚜鍔ㄦ洿鏂?.."
                fi

                ~/.acme.sh/acme.sh --upgrade --auto-upgrade
                if [ $? -ne 0 ]; then
                    LOGE "鑷姩鏇存柊璁剧疆澶辫触锛岃剼鏈€€鍑?.."
                    exit 1
                else
                    LOGI "璇佷功宸插畨瑁咃紝骞跺凡寮€鍚嚜鍔ㄧ画绛俱€?
                    ls -lah ${certPath}/${CF_Domain}
                    chmod 755 ${certPath}/${CF_Domain}
                fi
            fi
            show_menu
            ;;
        3)
            echo "姝ｅ湪閫€鍑?.."
            show_menu
            ;;
        *)
            echo "鏃犳晥閫夋嫨锛岃閲嶆柊閫夋嫨銆?
            show_menu
            ;;
    esac
}

generate_self_signed_cert() {
    cert_dir="/etc/sing-box"
    mkdir -p "$cert_dir"
    LOGI "璇烽€夋嫨璇佷功绫诲瀷锛?
    echo -e "${green}\t1.${plain} Ed25519锛堟帹鑽愶級"
    echo -e "${green}\t2.${plain} RSA 2048"
    echo -e "${green}\t3.${plain} RSA 4096"
    echo -e "${green}\t4.${plain} ECDSA prime256v1"
    echo -e "${green}\t5.${plain} ECDSA secp384r1"
    read -p "璇疯緭鍏ヤ綘鐨勯€夋嫨 [1-5锛岄粯璁?1]锛?" cert_type
    cert_type=${cert_type:-1}

    case "$cert_type" in
        1)
            algo="ed25519"
            key_opt="-newkey ed25519"
            ;;
        2)
            algo="rsa"
            key_opt="-newkey rsa:2048"
            ;;
        3)
            algo="rsa"
            key_opt="-newkey rsa:4096"
            ;;
        4)
            algo="ecdsa"
            key_opt="-newkey ec -pkeyopt ec_paramgen_curve:prime256v1"
            ;;
        5)
            algo="ecdsa"
            key_opt="-newkey ec -pkeyopt ec_paramgen_curve:secp384r1"
            ;;
        *)
            algo="ed25519"
            key_opt="-newkey ed25519"
            ;;
    esac

    LOGI "姝ｅ湪鐢熸垚鑷鍚嶈瘉涔︼紙$algo锛?.."
    sudo openssl req -x509 -nodes -days 3650 $key_opt \
        -keyout "${cert_dir}/self.key" \
        -out "${cert_dir}/self.crt" \
        -subj "/CN=myserver"
    if [[ $? -eq 0 ]]; then
        sudo chmod 600 "${cert_dir}/self."*
        LOGI "鑷鍚嶈瘉涔︾敓鎴愭垚鍔燂紒"
        LOGI "璇佷功璺緞锛?{cert_dir}/self.crt"
        LOGI "瀵嗛挜璺緞锛?{cert_dir}/self.key"
    else
        LOGE "鐢熸垚鑷鍚嶈瘉涔﹀け璐ャ€?
    fi
    before_show_menu
}

show_usage() {
    echo -e "S-UI 鎺у埗鑿滃崟鐢ㄦ硶"
    echo -e "------------------------------------------"
    echo -e "瀛愬懡浠わ細"
    echo -e "s-ui              - 绠＄悊鍛樼鐞嗚剼鏈?
    echo -e "s-ui start        - 鍚姩 s-ui"
    echo -e "s-ui stop         - 鍋滄 s-ui"
    echo -e "s-ui restart      - 閲嶅惎 s-ui"
    echo -e "s-ui status       - 鏌ョ湅 s-ui 褰撳墠鐘舵€?
    echo -e "s-ui enable       - 鍚敤寮€鏈鸿嚜鍚?
    echo -e "s-ui disable      - 绂佺敤寮€鏈鸿嚜鍚?
    echo -e "s-ui log          - 鏌ョ湅 s-ui 鏃ュ織"
    echo -e "s-ui update       - 鏇存柊"
    echo -e "s-ui install      - 瀹夎"
    echo -e "s-ui uninstall    - 鍗歌浇"
    echo -e "s-ui help         - 鎺у埗鑿滃崟鐢ㄦ硶"
    echo -e "------------------------------------------"
}

show_menu() {
  echo -e "
  ${green}S-UI 绠＄悊鑴氭湰 ${plain}
---------------------------------------------------------------
  ${green}0.${plain} 閫€鍑?
---------------------------------------------------------------
  ${green}1.${plain} 瀹夎
  ${green}2.${plain} 鏇存柊
  ${green}3.${plain} 鑷畾涔夌増鏈?
  ${green}4.${plain} 鍗歌浇
---------------------------------------------------------------
  ${green}5.${plain} 灏嗙鐞嗗憳璐﹀彿瀵嗙爜閲嶇疆涓洪粯璁ゅ€?
  ${green}6.${plain} 璁剧疆绠＄悊鍛樿处鍙峰瘑鐮?
  ${green}7.${plain} 鏌ョ湅绠＄悊鍛樿处鍙峰瘑鐮?
---------------------------------------------------------------
  ${green}8.${plain} 閲嶇疆闈㈡澘璁剧疆
  ${green}9.${plain} 璁剧疆闈㈡澘璁剧疆
  ${green}10.${plain} 鏌ョ湅闈㈡澘璁剧疆
---------------------------------------------------------------
  ${green}11.${plain} 鍚姩 S-UI
  ${green}12.${plain} 鍋滄 S-UI
  ${green}13.${plain} 閲嶅惎 S-UI
  ${green}14.${plain} 鏌ョ湅 S-UI 鐘舵€?
  ${green}15.${plain} 鏌ョ湅 S-UI 鏃ュ織
  ${green}16.${plain} 鍚敤 S-UI 寮€鏈鸿嚜鍚?
  ${green}17.${plain} 绂佺敤 S-UI 寮€鏈鸿嚜鍚?
---------------------------------------------------------------
  ${green}18.${plain} 鍚敤鎴栫鐢?BBR
  ${green}19.${plain} SSL 璇佷功绠＄悊
  ${green}20.${plain} Cloudflare SSL 璇佷功
---------------------------------------------------------------
 "
    show_status s-ui
    echo && read -p "璇疯緭鍏ヤ綘鐨勯€夋嫨 [0-20]锛?" num

    case "${num}" in
    0)
        exit 0
        ;;
    1)
        check_uninstall && install
        ;;
    2)
        check_install && update
        ;;
    3)
        check_install && custom_version
        ;;
    4)
        check_install && uninstall
        ;;
    5)
        check_install && reset_admin
        ;;
    6)
        check_install && set_admin
        ;;
    7)
        check_install && view_admin
        ;;
    8)
        check_install && reset_setting
        ;;
    9)
        check_install && set_setting
        ;;
    10)
        check_install && view_setting
        ;;
    11)
        check_install && start s-ui
        ;;
    12)
        check_install && stop s-ui
        ;;
    13)
        check_install && restart s-ui
        ;;
    14)
        check_install && status s-ui
        ;;
    15)
        check_install && show_log s-ui
        ;;
    16)
        check_install && enable s-ui
        ;;
    17)
        check_install && disable s-ui
        ;;
    18)
        bbr_menu
        ;;
    19)
        ssl_cert_issue_main
        ;;
    20)
        ssl_cert_issue_CF
        ;;
    *)
        LOGE "璇疯緭鍏ユ纭殑鏁板瓧 [0-20]"
        ;;
    esac
}

if [[ $# > 0 ]]; then
    case $1 in
    "start")
        check_install 0 && start s-ui 0
        ;;
    "stop")
        check_install 0 && stop s-ui 0
        ;;
    "restart")
        check_install 0 && restart s-ui 0
        ;;
    "status")
        check_install 0 && status 0
        ;;
    "enable")
        check_install 0 && enable s-ui 0
        ;;
    "disable")
        check_install 0 && disable s-ui 0
        ;;
    "log")
        check_install 0 && show_log s-ui 0
        ;;
    "update")
        check_install 0 && update 0
        ;;
    "install")
        check_uninstall 0 && install 0
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
