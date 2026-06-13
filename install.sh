#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# 妫€鏌?root 鏉冮檺
[[ $EUID -ne 0 ]] && echo -e "${red}鑷村懡閿欒锛?{plain}璇蜂娇鐢?root 鏉冮檺杩愯姝よ剼鏈?\n " && exit 1

# 妫€鏌ョ郴缁熷苟璁剧疆 release 鍙橀噺
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

arch() {
    case "$(uname -m)" in
    x86_64 | x64 | amd64) echo 'amd64' ;;
    i*86 | x86) echo '386' ;;
    armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
    armv7* | armv7 | arm) echo 'armv7' ;;
    armv6* | armv6) echo 'armv6' ;;
    armv5* | armv5) echo 'armv5' ;;
    s390x) echo 's390x' ;;
    *) echo -e "${green}涓嶆敮鎸佺殑 CPU 鏋舵瀯锛?{plain}" && rm -f install.sh && exit 1 ;;
    esac
}

echo "鏋舵瀯锛?(arch)"

install_base() {
    case "${release}" in
    centos | almalinux | rocky | oracle)
        yum -y update && yum install -y -q wget curl tar tzdata
        ;;
    fedora)
        dnf -y update && dnf install -y -q wget curl tar tzdata
        ;;
    arch | manjaro | parch)
        pacman -Syu && pacman -Syu --noconfirm wget curl tar tzdata
        ;;
    opensuse-tumbleweed)
        zypper refresh && zypper -q install -y wget curl tar timezone
        ;;
    *)
        apt-get update && apt-get install -y -q wget curl tar tzdata
        ;;
    esac
}

config_after_install() {
    echo -e "${yellow}姝ｅ湪杩佺Щ... ${plain}"
    /usr/local/s-ui/sui migrate

    echo -e "${yellow}瀹夎/鏇存柊瀹屾垚锛佸嚭浜庡畨鍏ㄨ€冭檻锛屽缓璁慨鏀归潰鏉胯缃?${plain}"
    read -p "鏄惁缁х画淇敼璁剧疆 [y/n]锛?: config_confirm
    if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
        echo -e "璇疯緭鍏?{yellow}闈㈡澘绔彛${plain}锛堢暀绌哄垯浣跨敤鐜版湁/榛樿鍊硷級锛?
        read config_port
        echo -e "璇疯緭鍏?{yellow}闈㈡澘璺緞${plain}锛堢暀绌哄垯浣跨敤鐜版湁/榛樿鍊硷級锛?
        read config_path

        # 璁㈤槄閰嶇疆
        echo -e "璇疯緭鍏?{yellow}璁㈤槄绔彛${plain}锛堢暀绌哄垯浣跨敤鐜版湁/榛樿鍊硷級锛?
        read config_subPort
        echo -e "璇疯緭鍏?{yellow}璁㈤槄璺緞${plain}锛堢暀绌哄垯浣跨敤鐜版湁/榛樿鍊硷級锛?
        read config_subPath

        # 璁剧疆閰嶇疆
        echo -e "${yellow}姝ｅ湪鍒濆鍖栵紝璇风◢鍊?..${plain}"
        params=""
        [ -z "$config_port" ] || params="$params -port $config_port"
        [ -z "$config_path" ] || params="$params -path $config_path"
        [ -z "$config_subPort" ] || params="$params -subPort $config_subPort"
        [ -z "$config_subPath" ] || params="$params -subPath $config_subPath"
        /usr/local/s-ui/sui setting ${params}

        read -p "鏄惁淇敼绠＄悊鍛樿处鍙峰瘑鐮?[y/n]锛?: admin_confirm
        if [[ "${admin_confirm}" == "y" || "${admin_confirm}" == "Y" ]]; then
            # 棣栦釜绠＄悊鍛樿处鍙峰瘑鐮?
            read -p "璇疯缃敤鎴峰悕锛? config_account
            read -p "璇疯缃瘑鐮侊細" config_password

            # 璁剧疆璐﹀彿瀵嗙爜
            echo -e "${yellow}姝ｅ湪鍒濆鍖栵紝璇风◢鍊?..${plain}"
            /usr/local/s-ui/sui admin -username ${config_account} -password ${config_password}
        else
            echo -e "${yellow}褰撳墠绠＄悊鍛樿处鍙峰瘑鐮侊細${plain}"
            /usr/local/s-ui/sui admin -show
        fi
    else
        echo -e "${red}宸插彇娑?..${plain}"
        if [[ ! -f "/usr/local/s-ui/db/s-ui.db" ]]; then
            local usernameTemp=$(head -c 6 /dev/urandom | base64)
            local passwordTemp=$(head -c 6 /dev/urandom | base64)
            echo -e "杩欐槸鍏ㄦ柊瀹夎锛屽嚭浜庡畨鍏ㄨ€冭檻灏嗙敓鎴愰殢鏈虹櫥褰曚俊鎭細"
            echo -e "###############################################"
            echo -e "${green}鐢ㄦ埛鍚嶏細${usernameTemp}${plain}"
            echo -e "${green}瀵嗙爜锛?{passwordTemp}${plain}"
            echo -e "###############################################"
            echo -e "${red}濡傛灉蹇樿鐧诲綍淇℃伅锛屽彲浠ヨ緭鍏?${green}s-ui${red} 鎵撳紑閰嶇疆鑿滃崟${plain}"
            /usr/local/s-ui/sui admin -username ${usernameTemp} -password ${passwordTemp}
        else
            echo -e "${red}杩欐槸鍗囩骇瀹夎锛屽皢淇濈暀鏃ц缃紱濡傛灉蹇樿鐧诲綍淇℃伅锛屽彲浠ヨ緭鍏?${green}s-ui${red} 鎵撳紑閰嶇疆鑿滃崟${plain}"
        fi
    fi
}

prepare_services() {
    if [[ -f "/etc/systemd/system/sing-box.service" ]]; then
        echo -e "${yellow}姝ｅ湪鍋滄 sing-box 鏈嶅姟... ${plain}"
        systemctl stop sing-box
        rm -f /usr/local/s-ui/bin/sing-box /usr/local/s-ui/bin/runSingbox.sh /usr/local/s-ui/bin/signal
    fi
    if [[ -e "/usr/local/s-ui/bin" ]]; then
        echo -e "###############################################################"
        echo -e "${green}/usr/local/s-ui/bin${red} 鐩綍宸插瓨鍦紒"
        echo -e "璇锋鏌ュ叾涓唴瀹癸紝骞跺湪杩佺Щ鍚庢墜鍔ㄥ垹闄?${plain}"
        echo -e "###############################################################"
    fi
    systemctl daemon-reload
}

install_s-ui() {
    cd /tmp/

    if [ $# == 0 ]; then
        last_version=$(curl -Ls "https://api.github.com/repos/Hhz0823/1s-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}鑾峰彇 s-ui 鐗堟湰澶辫触锛屽彲鑳芥槸 Github API 闄愬埗瀵艰嚧锛岃绋嶅悗閲嶈瘯${plain}"
            exit 1
        fi
        echo -e "宸茶幏鍙?s-ui 鏈€鏂扮増鏈細${last_version}锛屽紑濮嬪畨瑁?.."
        wget -N --no-check-certificate -O /tmp/s-ui-linux-$(arch).tar.gz https://github.com/Hhz0823/1s-ui/releases/download/${last_version}/s-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}涓嬭浇 s-ui 澶辫触锛岃纭鏈嶅姟鍣ㄥ彲浠ヨ闂?Github ${plain}"
            exit 1
        fi
    else
        last_version=$1
        [[ "${last_version}" != v* ]] && last_version="v${last_version}"
        url="https://github.com/Hhz0823/1s-ui/releases/download/${last_version}/s-ui-linux-$(arch).tar.gz"
        echo -e "寮€濮嬪畨瑁?s-ui ${last_version}"
        wget -N --no-check-certificate -O /tmp/s-ui-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}涓嬭浇 s-ui ${last_version} 澶辫触锛岃妫€鏌ヨ鐗堟湰鏄惁瀛樺湪${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/s-ui/ ]]; then
        systemctl stop s-ui
    fi

    tar zxvf s-ui-linux-$(arch).tar.gz
    rm s-ui-linux-$(arch).tar.gz -f

    chmod +x s-ui/sui s-ui/s-ui.sh
    cp s-ui/s-ui.sh /usr/bin/s-ui
    cp -rf s-ui /usr/local/
    cp -f s-ui/*.service /etc/systemd/system/
    rm -rf s-ui

    config_after_install
    prepare_services

    systemctl enable s-ui --now

    echo -e "${green}s-ui ${last_version}${plain} 瀹夎瀹屾垚锛岀幇宸插惎鍔ㄥ苟杩愯..."
    echo -e "浣犲彲浠ラ€氳繃浠ヤ笅 URL 璁块棶闈㈡澘锛?{green}"
    /usr/local/s-ui/sui uri
    echo -e "${plain}"
    echo -e ""
    s-ui help
}

echo -e "${green}姝ｅ湪鎵ц...${plain}"
install_base
install_s-ui $1
