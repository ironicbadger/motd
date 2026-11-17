# Source from a login shell startup file.
case $- in
    *i*)
        if [ -n "${SSH_TTY:-}" ] && [ -t 1 ] && command -v motd >/dev/null 2>&1; then
            command motd show || :
        fi
        ;;
esac
