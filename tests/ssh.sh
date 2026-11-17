#!/usr/bin/env bash
set -euo pipefail

# Run as root on a Linux test host to give the SSH session an isolated account.
binary=${1:?pass the path to a built motd binary}
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
tmp=$(mktemp -d /tmp/motd-ssh.XXXXXXXX)
account="motd-test-${tmp##*.}"
account_created=false
cleanup() {
    if "$account_created"; then
        userdel "$account"
    fi
    rm -rf -- "$tmp"
}
trap cleanup EXIT

chmod 0755 "$tmp"
install -d -m 0755 "$tmp/home" "$tmp/bin"
install -m 0755 "$binary" "$tmp/bin/motd"
install -m 0644 "$repo/contrib/motd-ssh.sh" "$tmp/hook.sh"
useradd --no-create-home --home-dir "$tmp/home" --shell /bin/bash --password NP "$account"
account_created=true
chown "$account" "$tmp/home"
cat > "$tmp/home/.bash_profile" <<EOF
export PATH="$tmp/bin:\$PATH"
. "$tmp/hook.sh"
EOF
# Bash may also read .bashrc for commands invoked by sshd.
cp "$tmp/home/.bash_profile" "$tmp/home/.bashrc"
install -d -m 0755 "$tmp/home/.config/motd"
cat > "$tmp/home/.config/motd/config.toml" <<'EOF'
hostname = "motd-ssh-test"
artwork = "none"
hide_cpu_temp = true
EOF

ssh-keygen -q -t ed25519 -N '' -f "$tmp/host_key"
ssh-keygen -q -t ed25519 -N '' -f "$tmp/client_key"
printf 'motd-test %s\n' "$(cat "$tmp/host_key.pub")" > "$tmp/known_hosts"
install -D -m 0644 "$tmp/client_key.pub" "$tmp/home/.ssh/authorized_keys"
cat > "$tmp/sshd_config" <<EOF
HostKey $tmp/host_key
AuthorizedKeysFile .ssh/authorized_keys
AllowUsers $account
PasswordAuthentication no
KbdInteractiveAuthentication no
UsePAM no
PrintMotd no
PrintLastLog no
EOF
if [[ ! -d /run/sshd ]]; then
    install -d -m 0755 /run/sshd
fi

# Run a private sshd over the client's pipes. No listening port is needed.
ssh_client=(
    ssh -F /dev/null -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes
    -o "UserKnownHostsFile=$tmp/known_hosts"
    -o "ProxyCommand=/usr/sbin/sshd -i -e -f $tmp/sshd_config"
    -i "$tmp/client_key" -l "$account"
)

printf 'exit\n' | TERM=dumb "${ssh_client[@]}" -tt motd-test > "$tmp/interactive"
if [[ $(grep -c '^Hostname: .*motd-ssh-test' "$tmp/interactive") != 1 ]]; then
    cat "$tmp/interactive" >&2
    exit 1
fi

"${ssh_client[@]}" -T motd-test 'printf "remote output\n"' > "$tmp/non-interactive"
printf 'remote output\n' | cmp - "$tmp/non-interactive"

# A remote command stays non-interactive even when the client requests a TTY.
TERM=dumb "${ssh_client[@]}" -tt motd-test 'printf "remote output\n"' </dev/null > "$tmp/command-with-tty"
printf 'remote output\r\n' | cmp - "$tmp/command-with-tty"
