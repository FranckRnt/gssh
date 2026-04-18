package completion

import (
	"fmt"
	"io"
)

// Bash outputs a bash completion script for gssh.
func Bash(w io.Writer) {
	fmt.Fprint(w, `_gssh() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="-l -u -c -f -k -p -t -w -r -v -n -o -insecure -y -known-hosts -h"

    case "${prev}" in
        -l|-f|-k|-known-hosts)
            COMPREPLY=( $(compgen -f -- "${cur}") )
            return 0
            ;;
        -o)
            COMPREPLY=( $(compgen -W "text json" -- "${cur}") )
            return 0
            ;;
        -u|-c|-p|-t|-w|-r)
            return 0
            ;;
    esac

    if [[ "${cur}" == -* ]]; then
        COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
        return 0
    fi
}
complete -F _gssh gssh
`)
}

// Zsh outputs a zsh completion script for gssh.
func Zsh(w io.Writer) {
	fmt.Fprint(w, `#compdef gssh

_gssh() {
    _arguments \
        '-l[Server list file]:file:_files' \
        '-u[SSH user]:user:' \
        '*-c[Command to execute]:command:' \
        '-f[Script file with commands]:file:_files' \
        '-k[SSH private key path]:file:_files' \
        '-known-hosts[Known hosts file]:file:_files' \
        '-p[SSH port]:port:' \
        '-t[Timeout per server]:duration:' \
        '-w[Max concurrent workers]:count:' \
        '-r[Number of retries]:count:' \
        '-v[Verbose output]' \
        '-n[Dry run]' \
        '-o[Output format]:format:(text json)' \
        '-insecure[Skip host key verification]' \
        '-y[Confirm dangerous operations]'
}

_gssh "$@"
`)
}

// Fish outputs a fish completion script for gssh.
func Fish(w io.Writer) {
	fmt.Fprint(w, `complete -c gssh -s l -d "Server list file" -r -F
complete -c gssh -s u -d "SSH user" -r
complete -c gssh -s c -d "Command to execute" -r
complete -c gssh -s f -d "Script file with commands" -r -F
complete -c gssh -s k -d "SSH private key path" -r -F
complete -c gssh -l known-hosts -d "Known hosts file" -r -F
complete -c gssh -s p -d "SSH port" -r
complete -c gssh -s t -d "Timeout per server" -r
complete -c gssh -s w -d "Max concurrent workers" -r
complete -c gssh -s r -d "Number of retries" -r
complete -c gssh -s v -d "Verbose output"
complete -c gssh -s n -d "Dry run"
complete -c gssh -s o -d "Output format" -r -a "text json"
complete -c gssh -l insecure -d "Skip host key verification"
complete -c gssh -s y -d "Confirm dangerous operations"
`)
}
