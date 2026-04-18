# gssh — SSH at scale

Exécution de commandes SSH en parallèle sur plusieurs serveurs. Outil rapide et léger, écrit en Go.

## Fonctionnalités

- Exécution parallèle avec worker pool configurable
- Multi-commandes sur une seule connexion SSH par serveur (`-c cmd1 -c cmd2`)
- Chargement de commandes depuis un fichier script (`-f`)
- **Transfert de fichiers** : upload (`push`) et download (`pull`) via SFTP
- **Bastion/jump host** : flag `-J user@host[:port]` pour tunneler via un proxy SSH
- **Tags et filtrage** : tag des serveurs avec `#tag` et filtrage avec `-g`
- **Inventaire dynamique** : source de serveurs depuis un fichier, un script exécutable ou une URL
- **Sudo** : flag `-S` pour exécuter les commandes via `sudo` (mot de passe demandé une seule fois)
- **Templates de commandes** : variables `{{.Hostname}}`, `{{.Port}}`, `{{.Tags}}`, etc. dans les commandes
- **Streaming en temps réel** : flag `-s` pour afficher la sortie ligne par ligne au fur et à mesure (style `tail -f`)
- **Diff mode** : flag `--diff` pour comparer les sorties entre serveurs et détecter les drifts
- **Sortie groupée** : flag `--group` pour regrouper les serveurs par sortie identique
- **Rapport HTML** : flag `--report html` pour générer un rapport HTML statique avec tableau triable, dark/light mode
- Retry automatique des serveurs en échec
- Progress bar en temps réel
- Sortie texte colorée ou JSON
- Détection automatique des clés SSH (ed25519, ecdsa, rsa)
- Support de l'agent SSH et des clés avec passphrase
- Résolution DNS en amont (cache)
- Protection mémoire (sortie capturée limitée à 1 Mo par flux)
- Fichier de configuration `~/.gssh.yaml`
- Complétion shell (bash, zsh, fish)
- Vérification des host keys via `known_hosts`

## Installation

### Depuis les releases GitHub

Télécharger le binaire correspondant à votre OS depuis la page [Releases](https://github.com/FranckRnt/gssh/releases).

### Depuis les sources

```bash
go install github.com/FranckRnt/gssh/cmd/gssh@latest
```

Ou cloner et compiler :

```bash
git clone git@github.com:FranckRnt/gssh.git
cd gssh
go build -o gssh ./cmd/gssh/
```

## Commandes

gssh utilise des sous-commandes :

| Commande | Description |
|----------|-------------|
| `gssh run` | Exécuter des commandes SSH (par défaut, `run` est optionnel) |
| `gssh push` | Uploader un fichier vers N serveurs via SFTP |
| `gssh pull` | Télécharger un fichier depuis N serveurs via SFTP |

## Utilisation rapide

### Exécution de commandes

```bash
# Lancer une commande sur tous les serveurs
gssh -l servers.txt -u root -c "uptime"

# Équivalent avec sous-commande explicite
gssh run -l servers.txt -u root -c "uptime"

# Plusieurs commandes (réutilise la connexion SSH)
gssh -l servers.txt -u deploy -c "uptime" -c "df -h" -c "free -m"

# Charger les commandes depuis un fichier
gssh -l servers.txt -u root -f commands.txt

# Mode dry-run : voir les serveurs ciblés sans rien exécuter
gssh -l servers.txt -u root -c "uptime" -n
```

### Bastion / jump host

```bash
# Exécuter via un bastion
gssh -l servers.txt -u root -c "uptime" -J admin@bastion.example.com

# Bastion avec port personnalisé
gssh -l servers.txt -u root -c "uptime" -J admin@bastion.example.com:2222

# Upload via bastion
gssh push -l servers.txt -u root -s ./config.yml -d /etc/app/config.yml \
     -J admin@bastion.example.com

# Download via bastion
gssh pull -l servers.txt -u root -s /var/log/app.log -d ./logs/ \
     -J admin@bastion.example.com
```

Le bastion utilise les mêmes méthodes d'authentification (agent SSH, clé privée) et la même vérification des host keys que les serveurs cibles.

### Sudo

```bash
# Exécuter une commande avec sudo (le mot de passe est demandé une seule fois)
gssh -l servers.txt -u deploy -c "apt update" -S

# Sudo + multi-commandes
gssh -l servers.txt -u deploy -c "apt update" -c "apt upgrade -y" -S

# Sudo + bastion
gssh -l servers.txt -u deploy -c "systemctl restart nginx" -S -J admin@bastion
```

Le mot de passe est saisi de manière sécurisée (sans écho) et envoyé à chaque commande via `sudo -S`.

### Templates de commandes

Les commandes peuvent contenir des variables Go templates qui sont résolues par serveur :

| Variable | Description | Exemple |
|----------|-------------|---------|
| `{{.Hostname}}` | Nom d'hôte (sans port) | `web01.example.com` |
| `{{.Host}}` | Entrée complète (avec port si présent) | `web01.example.com:2222` |
| `{{.IP}}` | Alias de Hostname | `web01.example.com` |
| `{{.Port}}` | Port SSH | `2222` |
| `{{.Tags}}` | Tableau de tags | `[web prod]` |
| `{{.TagsCSV}}` | Tags séparés par des virgules | `web,prod` |

```bash
# Créer un fichier avec le nom du serveur
gssh -l servers.txt -u root -c "echo {{.Hostname}} > /etc/hostname"

# Configurer selon les tags
gssh -l servers.txt -u root -c "echo 'role={{.TagsCSV}}' >> /etc/environment"

# Utiliser le port dans une commande
gssh -l servers.txt -u root -c "echo 'SSH port: {{.Port}}'"
```

Les templates ne sont évalués que si au moins une commande contient `{{`. Si aucune commande ne contient de template, il n'y a aucun overhead.

### Transfert de fichiers

```bash
# Upload un fichier vers tous les serveurs
gssh push -l servers.txt -u root -s ./nginx.conf -d /etc/nginx/nginx.conf

# Upload avec port et clé personnalisés
gssh push -l servers.txt -u deploy -s ./app.tar.gz -d /opt/app.tar.gz \
     -p 2222 -k ~/.ssh/deploy_key

# Download un fichier depuis tous les serveurs
# Les fichiers sont sauvegardés dans ./logs/<hostname>/syslog
gssh pull -l servers.txt -u root -s /var/log/syslog -d ./logs/

# Download des configs nginx
gssh pull -l servers.txt -u deploy -s /etc/nginx/nginx.conf -d ./configs/
```

## Fichier de serveurs

Un fichier texte avec un serveur par ligne. Les lignes vides et les commentaires (`#` en début de ligne) sont ignorés. Le port peut être précisé avec `:port`. Les tags sont optionnels, préfixés par `#` après le nom d'hôte.

```
# servers.txt
web01.example.com #web #prod
web02.example.com:2222 #web #staging
10.0.1.50 #db #prod
db01.example.com #db #prod #paris
plain-host
```

> Le fichier ne doit pas être world-writable (permissions `0644` ou plus restrictif).

### Filtrage par tags

```bash
# Cibler uniquement les serveurs web
gssh -l servers.txt -u root -c "uptime" -g web

# Cibler les serveurs web ET prod (intersection)
gssh -l servers.txt -u root -c "uptime" -g web,prod

# Fonctionne aussi avec push/pull
gssh push -l servers.txt -u root -s ./nginx.conf -d /etc/nginx/nginx.conf -g web
```

Le flag `-g` prend une liste de tags séparés par des virgules. Les serveurs doivent avoir **tous** les tags spécifiés pour être sélectionnés.

### Inventaire dynamique

Au lieu d'un fichier statique, la source de serveurs (`-l`) peut être :

**Un script exécutable** — gssh détecte les fichiers avec le bit exécutable et exécute le script. La sortie stdout est parsée au même format (un serveur par ligne, avec tags optionnels).

```bash
# inventory.sh doit être exécutable (chmod +x)
gssh -l ./inventory.sh -u root -c "uptime"

# Exemple de script
#!/bin/bash
echo "web01.example.com #web #prod"
echo "web02.example.com #web #staging"
# Peut interroger une API, une base de données, AWS, etc.
```

**Une URL HTTP/HTTPS** — gssh fetch l'URL et parse le body au même format.

```bash
gssh -l https://inventory.example.com/servers -u root -c "uptime"
gssh -l https://inventory.example.com/servers -u root -c "uptime" -g prod
```

## Options

### Commande `run` (ou sans sous-commande)

| Flag | Description | Défaut |
|------|-------------|--------|
| `-l <fichier>` | Fichier de liste de serveurs **(requis)** | — |
| `-u <user>` | Utilisateur SSH **(requis)** | — |
| `-c <commande>` | Commande à exécuter (répétable) **(requis, exclusif avec `-f`)** | — |
| `-f <fichier>` | Fichier de commandes, une par ligne **(requis, exclusif avec `-c`)** | — |
| `-k <chemin>` | Clé privée SSH | auto-détection |
| `-p <port>` | Port SSH par défaut | `22` |
| `-t <durée>` | Timeout par serveur | `30s` |
| `-w <nombre>` | Workers SSH simultanés max | `100` |
| `-r <nombre>` | Nombre de tentatives en cas d'échec | `0` |
| `-v` | Sortie en temps réel (verbose) | `false` |
| `-n` | Dry run | `false` |
| `-o <format>` | Format de sortie : `text` ou `json` | `text` |
| `-known-hosts <chemin>` | Fichier known_hosts | `~/.ssh/known_hosts` |
| `-insecure` | Désactiver la vérification des host keys | `false` |
| `-y` | Confirmer les opérations dangereuses (requis avec `-insecure`) | `false` |
| `-J <user@host[:port]>` | Bastion/jump host pour tunneler les connexions | — |
| `-g <tags>` | Filtrer par tags (séparés par des virgules, intersection) | — |
| `-S` | Exécuter les commandes via sudo (demande le mot de passe une fois) | `false` |
| `-L <chemin>` | Répertoire pour les fichiers de log JSON | `~/.gssh/logs/` |
| `-s` | Streaming en temps réel, ligne par ligne (implique `-v`) | `false` |
| `--diff` | Comparer les sorties entre serveurs après exécution | `false` |
| `--group` | Grouper les serveurs par sortie identique | `false` |
| `--report <format>` | Générer un rapport : `html` | — |

> **Note** : `-c` et `-f` sont mutuellement exclusifs. Utilisez l'un ou l'autre, pas les deux.

### Commandes `push` et `pull`

| Flag | Description | Défaut |
|------|-------------|--------|
| `-l <fichier>` | Fichier de liste de serveurs **(requis)** | — |
| `-u <user>` | Utilisateur SSH **(requis)** | — |
| `-s <chemin>` | Chemin source **(requis)** | — |
| `-d <chemin>` | Chemin destination **(requis)** | — |
| `-k <chemin>` | Clé privée SSH | auto-détection |
| `-p <port>` | Port SSH par défaut | `22` |
| `-w <nombre>` | Workers SSH simultanés max | `100` |
| `-v` | Sortie verbose | `false` |
| `-n` | Dry run | `false` |
| `-o <format>` | Format de sortie : `text` ou `json` | `text` |
| `-known-hosts <chemin>` | Fichier known_hosts | `~/.ssh/known_hosts` |
| `-insecure` | Désactiver la vérification des host keys | `false` |
| `-y` | Confirmer les opérations dangereuses | `false` |
| `-J <user@host[:port]>` | Bastion/jump host pour tunneler les connexions | — |
| `-g <tags>` | Filtrer par tags (séparés par des virgules, intersection) | — |
| `-L <chemin>` | Répertoire pour les fichiers de log JSON | `~/.gssh/logs/` |

Pour `push`, `-s` est le chemin local et `-d` le chemin distant. Si `-d` est un répertoire existant sur le serveur (ou finit par `/`), le nom du fichier source est ajouté automatiquement (ex : `-s ./nginx.conf -d /etc/nginx/` → `/etc/nginx/nginx.conf`).
Pour `pull`, `-s` est le chemin distant et `-d` le répertoire local de sortie (les fichiers sont sauvegardés dans `<dest>/<hostname>/<filename>`).

## Exemples

### Sortie verbose avec retries et format JSON

```bash
gssh -l servers.txt -u root -c "systemctl status nginx" -v -r 3 -o json
```

### Clé SSH spécifique, port et timeout personnalisés

```bash
gssh -l servers.txt -u deploy -c "systemctl restart app" \
     -k ~/.ssh/deploy_key -p 2222 -t 60s -w 50
```

### Fichier de commandes

```bash
# commands.txt
uptime
df -h
free -m
systemctl status nginx
```

```bash
gssh -l servers.txt -u root -f commands.txt
```

Les commandes sont exécutées dans l'ordre sur chaque serveur. Si une commande échoue, les suivantes sont ignorées pour ce serveur.

### Upload d'un fichier de configuration

```bash
gssh push -l servers.txt -u root -s ./nginx.conf -d /etc/nginx/nginx.conf
```

### Collecte de logs depuis tous les serveurs

```bash
gssh pull -l servers.txt -u root -s /var/log/syslog -d ./collected-logs/
# Résultat : ./collected-logs/web01/syslog, ./collected-logs/web02/syslog, etc.
```

## Modes de sortie

### Mode normal (par défaut)

Une ligne compacte par serveur avec le statut de chaque commande. Le détail (error, stderr) n'est affiché que pour les commandes en échec.

```
192.168.1.12  hostname OK | uptime OK | yum check-update FAIL (exit=100)
    error: [yum check-update] exit status 100
192.168.1.13  hostname OK | uptime OK | yum check-update OK

── Summary ──────────────────────────
  Total:    6
  Success:  5
  Failed:   1
  Duration: 1.5s
```

Pour les transferts :

```
web01  push OK  4.2 KiB  230ms
web02  push OK  4.2 KiB  245ms
web03  push FAIL  connection refused  50ms

── Transfer Summary ─────────────────
  Total:      3
  Success:    2
  Failed:     1
  Bytes:      8.4 KiB
  Duration:   280ms
```

### Mode verbose (`-v`)

Sortie complète de chaque commande en temps réel, avec stdout et stderr affichés intégralement.

```bash
gssh -l servers.txt -u root -c "uptime" -c "df -h" -v
```

### Mode streaming (`-s`)

Affiche la sortie ligne par ligne au fur et à mesure de l'exécution, comme `tail -f`. Chaque ligne est préfixée par le hostname. Utile pour les commandes longues (`apt upgrade`, `docker pull`).

```bash
gssh -l servers.txt -u root -c "apt update && apt upgrade -y" -s
```

```
web01 Hit:1 http://deb.debian.org/debian bookworm InRelease
web02 Hit:1 http://deb.debian.org/debian bookworm InRelease
web01 Reading package lists...
web02 err| W: Some warning here
```

Les lignes stderr sont marquées avec le préfixe `err|` en jaune.

### Mode diff (`--diff`)

Compare les sorties de chaque commande entre tous les serveurs. Utile pour détecter des drifts de configuration.

```bash
gssh -l servers.txt -u root -c "cat /etc/hostname" --diff
```

```
── Diff: cat /etc/hostname
  ⚠ 2 different outputs across 5 server(s)

  MAJORITY OK (3 server(s)) [web01, web02, web03]
    myhost

  GROUP OK (2 server(s)) [db01, db02]
    dbhost
```

### Mode groupé (`--group`)

Regroupe les serveurs par sortie identique au lieu de l'affichage par serveur. Montre directement combien de serveurs ont produit le même résultat.

```bash
gssh -l servers.txt -u root -c "systemctl is-active nginx" --group
```

```
▸ OK  42 server(s): [web01, web02, web03, ... +39 more]
  active

▸ FAIL (exit=3)  3 server(s): [db01, db02, db03]
  inactive
```

### Rapport HTML (`--report html`)

Génère un fichier HTML statique autonome avec un tableau triable, des couleurs, et un résumé visuel. Aucune dépendance externe. Le rapport inclut un toggle **dark/light mode** (détection automatique de la préférence système, persistance via `localStorage`).

```bash
gssh -l servers.txt -u root -c "uptime" -c "df -h" --report html
```

Le rapport est écrit dans le répertoire de logs (`~/.gssh/logs/` par défaut ou `-L`).

### Format JSON (`-o json`)

Le fichier de log JSON est toujours écrit. Avec `-o json`, le summary est aussi en JSON.

## Fichier de configuration

Créer `~/.gssh.yaml` pour définir des valeurs par défaut. Les flags en ligne de commande ont toujours la priorité.

```yaml
# ~/.gssh.yaml
user: deploy
key: ~/.ssh/deploy_key
port: "22"
timeout: "45s"
workers: 50
retries: 1
known_hosts: ~/.ssh/known_hosts
verbose: false
output: text
log_dir: ~/.gssh/logs/
```

## Complétion shell

```bash
# Bash (ajouter dans ~/.bashrc)
eval "$(gssh --completion=bash)"

# Zsh (ajouter dans ~/.zshrc)
eval "$(gssh --completion=zsh)"

# Fish
gssh --completion=fish | source
```

## Authentification SSH

gssh tente l'authentification dans cet ordre :

1. **Agent SSH** (`SSH_AUTH_SOCK`) — si l'agent est actif et contient des clés
2. **Clé privée fichier** — spécifiée avec `-k` ou auto-détectée dans `~/.ssh/` (ed25519, ecdsa, rsa)

Si la clé est protégée par une passphrase, gssh la demande interactivement.

## Fichier de log

Après chaque exécution, gssh écrit un fichier JSON `gssh-YYYY-MM-DD_HH-MM-SS.log` dans le répertoire de logs. Par défaut, ce répertoire est `~/.gssh/logs/`. Il peut être modifié via le flag `-L` ou la clé `log_dir` dans `~/.gssh.yaml`. Le répertoire est créé automatiquement s'il n'existe pas.

## Codes de sortie

| Code | Signification |
|------|---------------|
| `0` | Tous les serveurs ont réussi |
| `1` | Erreur de configuration ou d'initialisation |
| `2` | Au moins un serveur a échoué |

## Licence

MIT
