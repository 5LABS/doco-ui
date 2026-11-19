<img src="assets/icon.svg" width="64" height="64" alt="">

# doco-ui

Lokale Control UI für Docker und doco-cd. Die Übersicht zeigt Containerzustände,
Compose-Projekte und die letzten 30 Instanz-Läufe einschließlich ihrer Zielkontexte.
Über die Containerliste lassen sich gestoppte oder ungesunde Container finden,
Details einsehen, Logs verfolgen und CPU- und RAM-Werte beobachten. Projekte
und Netzwerke verlinken auf die zugehörigen Container.

## Betrieb mit Compose

Voraussetzungen sind Linux, Docker Engine mit Unterstützung für API v1.44
(Docker 25 oder neuer mit aktivierter v1.44), Docker Compose v2+ und doco-cd
ab 0.135.0. Das Beispiel bindet diese doco-cd-Version fest ein. Beide Dienste
greifen auf dieselbe Engine im Docker Context `default` zu.

Im Projektverzeichnis einmalig die Secrets für eine **neue** Installation anlegen:

```sh
mkdir -m 700 secrets
(umask 077; openssl rand -hex 32 > secrets/ui_password)
(umask 077; openssl rand -hex 32 > secrets/doco_key)
(umask 077; printf -- '- key: "%s"\n  role: inspect\n' "$(cat secrets/doco_key)" > secrets/doco_keys.yaml)
# Compose bindet lokale Secret-Dateien ein. Der UI-Prozess läuft als UID 65532.
sudo chown 65532 secrets/ui_password secrets/doco_key
printf '[]\n' > sources.json
export DOCKER_GID="$(stat -c %g /var/run/docker.sock)"
docker compose up -d --build
```

Unter http://localhost:8080 mit Benutzer `admin` und dem Wert aus
`secrets/ui_password` anmelden. Das UI-Passwort schützt auch Lesezugriffe und
Logs. Für Zugriff über das Netzwerk einen HTTPS-Reverse-Proxy vorschalten.
Der Socket gewährt dem Backend weitreichenden Hostzugriff. Der Schreibmodus der
UI ist standardmäßig ausgeschaltet.

Bei einer vorhandenen doco-cd-Instanz nur den UI-Dienst übernehmen, dem
gemeinsamen Docker-Netz hinzufügen und `DOCO_URL` auf deren interne Basis-URL
setzen. Der API-Schlüssel bleibt im Backend. Das Docker-Inventar funktioniert
auch bei Ausfall von doco-cd.

## Konfiguration

| Variable | Bedeutung |
| --- | --- |
| UI_PASSWORD | Pflicht. Passwort für den Benutzer admin |
| DOCKER_SOCKET | Unix-Socket der lokalen Engine, Standard /var/run/docker.sock |
| DOCO_URL, DOCO_KEY | Basis-URL ohne API-Pfad und Schlüssel für doco-cd |
| UI_WRITE | true erlaubt Containeraktionen und schaltet die folgenden Freigaben frei |
| DOCO_CONTROL | true erlaubt Projektaktionen, benötigt projects:inspect und projects:control |
| DOCO_POLL | true erlaubt den Abgleich konfigurierter Quellen, benötigt poll:trigger und runs:read |
| GITOPS_SOURCES | JSON-Liste der Quellen, standardmäßig leer |

UI_PASSWORD, DOCO_KEY und GITOPS_SOURCES unterstützen jeweils die Variante
mit Suffix `_FILE`. Ist diese gesetzt, verwendet der Server den Dateiinhalt.
Für das Dashboard braucht der doco-cd-Schlüssel `projects:inspect` und
`runs:read`. Das Beispiel verwendet die Rolle `inspect`. Für beide
Schreibfreigaben kann die Rolle `write` oder eine explizite Berechtigungsliste
mit den oben genannten Rechten verwendet werden. Freigaben auf die tatsächlichen
Schlüsselrechte abstimmen und nach Konfigurationsänderungen die Dienste mit
`docker compose up -d --force-recreate` neu erstellen.

Für zwei auswählbare Quellen die Struktur aus [sources.example.json](sources.example.json)
in `sources.json` übernehmen und an die eigenen Repositories anpassen. Die
stabile ID identifiziert eine Quelle, `scope` beschreibt die betroffenen Projekte
für die Bestätigung. Diese Beschreibung bei Änderungen im Repository mitpflegen.
`reference` und doco-cd-`target` sind optional. Im Container beziehen sich
Git-Zugangsdaten für private Repositories auf den doco-cd-Dienst.

## Betriebsverhalten

- Projektaktionen erfordern einen Eintrag in der Projekt-API und bei allen
  zugehörigen Containern das Manager-Label `cd.doco.metadata.manager=doco-cd`.
  Containeraktionen sind auch für eigenständige Container verfügbar.
- Ein Abgleich sendet die ausgewählte Konfiguration an den asynchronen Poll-Endpunkt.
  Das Backend verfolgt die Job-ID auch nach Schließen des Tabs. doco-cd meldet
  den endgültigen Status. Laufzuordnung und Aktionssperren liegen im Speicher
  eines einzelnen UI-Prozesses. Nach einem Neustart die Historie prüfen.
- Inventarabfragen teilen einen vier Sekunden gültigen Cache. Eine geöffnete
  Containerdetailseite bezieht Statistiken über SSE, mit höchstens einer Messung
  pro fünf Sekunden und Container. Bis zu 60 Punkte je Container und bis zu 64
  Containerverläufe bleiben im Speicher. CPU entspricht der Auslastung in
  CPU-Kernen (ein belegter Kern = 100 %), RAM der von Docker gemeldeten
  Speichernutzung einschließlich Cache. Die Werte beziehen sich auf den Container.
- Logs starten mit bis zu 200 Zeilen. Jeder Abruf ist auf 256 KiB und
  65 Sekunden begrenzt. Zum Weiterverfolgen den Stream erneut öffnen.

## Entwicklung

Mit Node.js ab 22.12, npm und Go ab 1.23:

```sh
(cd web && npm ci && npm run build)
go test -race ./...
UI_PASSWORD_FILE=/pfad/zum/ui-passwort go run .
```

Vite läuft optional mit `cd web && npm run dev` und leitet API-Anfragen an
Port 8080 weiter. Für den Entwicklungsserver muss der Browser für die API am
gleichen Origin per HTTP Basic angemeldet sein. Der Produktionsbuild liefert
Frontend und API zusammen aus.

[Projektplan](docs/plan.md) · [doco-cd-API der Referenzversion](https://github.com/kimdre/doco-cd/blob/v0.135.0/wiki/docs/Endpoints/REST-API.md)
