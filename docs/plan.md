# Projektplan für doco-ui

Stand: 8. November 2026. Dieses Dokument beschreibt die geplante Anwendung.
Es ist noch keine Implementierung vorhanden.

## Ziel und abgestimmte Entscheidungen

Betreiber sollen erkennen können, welche Container laufen, zu welchen Projekten
und Netzwerken sie gehören, wie viele Ressourcen sie verbrauchen und ob
Deployments erfolgreich sind. Häufige Betriebsaktionen sollen direkt erreichbar sein.

- Einstieg mit einem lokalen Docker-Host und einer doco-cd-Instanz.
- Eigener Container neben doco-cd mit einem schlanken Go-Backend.
- React mit TypeScript, Tailwind CSS, shadcn/ui und TanStack Query im Frontend.
- Gruppierung nach Compose-Projekten.
- Alle Container des Hosts sind sichtbar, auch unabhängig von doco-cd.
- Weitere Hosts sollen später auf derselben Grundstruktur ergänzt werden können.

Als Arbeitsannahme sind auch Start, Stop und Restart für Container möglich, die
nicht durch doco-cd verwaltet werden. Die Verwaltung durch doco-cd muss zuverlässig
erkannt und in der Oberfläche ausgewiesen werden.

## Funktionsumfang der ersten Version

### Übersicht und Ressourcen

- Dashboard mit Containerzuständen, Zustand der Compose-Projekte und letzten
  Deployment-Läufen. Im Dashboard lassen sich konfigurierte GitOps-Quellen
  auswählen, abgleichen und anhand ihres Laufstatus verfolgen. Datenquelle und
  Verbindungszustand sind erkennbar.
- Containerliste mit Suche, Statusfiltern und Gruppierung nach Compose-Projekt.
  Gestoppte und fehlerhafte Container bleiben auffindbar.
- Containerdetails mit Image, Status, Healthcheck sofern vorhanden, Ports, Mounts,
  Netzwerken und begrenzten, bei Bedarf verfolgbaren Logs.
- Compose-Projekte mit Services, Containern, Verwaltungszuordnung und verfügbaren
  Deployment-Informationen.
- Netzwerke mit Treiber, Subnetzen und angeschlossenen Containern samt IP-Adressen.
- CPU- und RAM-Werte pro Container; Netzwerk- und Disk-I/O können ergänzt werden.
  Ein kurzer Verlauf wird zeitlich und mengenmäßig begrenzt im Arbeitsspeicher gehalten.

Containerwerte werden als solche bezeichnet. Gesamte Host-Auslastung ist ein
separater Messwert und darf nicht aus der Summe der Containerwerte abgeleitet werden.

### Aktionen

- Container starten, stoppen und neu starten.
- Compose-Projekte über die verfügbaren doco-cd-Endpunkte starten, stoppen und
  neu starten; unterstützte Projekte oder Services gezielt neu erstellen.
- Im Dashboard eine GitOps-Quelle auswählen und zusätzlich zu Webhooks und
  automatischem Polling einen Abgleich über doco-cd starten. Die Auswahl zeigt
  vor dem Start, welche Konfiguration abgeglichen wird und ob sie mehrere
  Projekte umfasst. Doco-cd deployt bei Bedarf und das Dashboard zeigt den
  tatsächlichen Laufstatus.
- Aktionen nach Ziel, Verwaltungszuordnung und API-Fähigkeiten anbieten.
  Compose-Gruppierung allein gilt nicht als Nachweis für doco-cd-Verwaltung.
- Ausstehende Aktionen anzeigen, doppelte Ausführung verhindern und nach Abschluss
  den tatsächlichen Zustand neu laden. Ein angenommener Auftrag ist noch kein Erfolg.

Die UI erklärt die Reichweite einer Aktion, etwa einzelner Container oder ganzes
Compose-Projekt. Ein temporärer Stop ändert nicht die in Git hinterlegte Konfiguration.

## Technischer Vorschlag

### Frontend

- Vite baut eine React-Anwendung mit TypeScript im Strict-Modus.
- Tailwind CSS und shadcn/ui bilden die Basis für Layout, Formulare, Dialoge,
  Menüs und Statusanzeigen. Komponenten werden nach Bedarf aufgenommen.
- TanStack Query verwaltet API-Daten, Cache, Aktualisierung und Mutationen.
- TanStack Router übernimmt Routen, Detailansichten sowie teilbare Such- und
  Filterparameter. API-Daten bleiben im Query-Cache.
- TanStack Table unterstützt Sortierung, Filter und Spaltensichtbarkeit in den
  Ressourcenlisten.
- Lokaler React-State reicht zunächst für kurzlebige Oberflächenzustände.
- Diagramme werden erst mit dem Statistikbereich ergänzt; die Bibliothek wird
  dann anhand des Umfangs gewählt.

Browserzugriffe laufen ausschließlich über die eigene Go-API derselben Anwendung.
Der doco-cd-API-Schlüssel und Docker-Zugangsdaten bleiben im Backend.

### Go-Backend und Betrieb

- Go stellt HTTP-API und statische Frontend-Dateien über denselben Port bereit.
- Der Frontend-Build wird mit `go:embed` in das Binary eingebunden.
- Ein mehrstufiger Container-Build erzeugt Frontend und Backend. Das Laufzeitimage
  enthält das Go-Binary und die benötigten Laufzeitdateien.
- doco-cd wird über seine HTTP-API im gemeinsamen Container-Netz erreicht.
- Die lokale Docker Engine wird über den bereitgestellten Unix-Socket erreicht.
- Getrennte Adapter für doco-cd und Docker führen Daten zu eigenen API-Modellen
  zusammen. Das Backend bietet gezielte Operationen an.
- Konfiguration und Zugangsdaten kommen aus Umgebungsvariablen beziehungsweise
  Secret-Dateien. Für den ersten Umfang ist keine zusätzliche Datenbank geplant.
- Das Backend hält die auswählbaren GitOps-Quellen mit Repository-URL und bei
  Bedarf Git-Referenz und doco-cd-Target vor. Es löst die ausgewählte Quelle auf,
  stößt den Poll-Lauf bei doco-cd an und fragt den Status über die zurückgegebene
  `job_id` ab.

Die konkrete Mindestversion von doco-cd wird vor der Integration festgelegt und
gegen deren API geprüft. Die jeweils aktuelle Dokumentation ist keine feste
Kompatibilitätszusage für ältere Installationen.

### Zuständigkeit der Datenquellen

- doco-cd: Deployment-Läufe, verfügbare Projektinformationen und unterstützte
  Compose- beziehungsweise Deployment-Aktionen.
- Docker Engine: Containerinventar, Laufzustand, Labels, Netzwerke, Logs,
  Containerstatistiken und einzelne Containeraktionen.
- doco-ui: Zusammenführung, Zielzuordnung, verfügbare Aktionen und kurzer Statistikverlauf.

Ein Ausfall einer Datenquelle wird separat ausgewiesen. Beispielsweise bleibt das
Docker-Inventar bei nicht erreichbarem doco-cd nutzbar; betroffene Aktionen sind
mit einer Begründung nicht verfügbar.

### Aktualisierung und Ressourcenverbrauch

Für den Einstieg reichen gezieltes Polling für Übersichten und Laufstatus sowie
separate Streams für geöffnete Logs. Für Live-Statistiken und spätere Ereignisse
ist Server-Sent Events vorgesehen.

Das Backend teilt Inventar- und Statistikerfassung zwischen Browsern und begrenzt
Abtastrate, Aufbewahrung, Logumfang und Warteschlangen. Beim Trennen einer Verbindung
werden zugehörige Streams beendet. Nach Wiederverbindung wird der Zustand neu geladen.
Ereignisse können gezielt Query-Einträge aktualisieren oder ungültig machen.

## Vorbereitung auf mehrere Hosts

Ein Ziel besitzt eine stabile ID und verbindet Docker-Endpunkt, doco-cd-Instanz
und Docker Context. In Version 1 gibt es nur das lokale Ziel.

Ressourcen, Aktionen und Query-Keys enthalten von Anfang an die Ziel-ID. Damit
können gleichnamige Projekte auf unterschiedlichen Hosts eindeutig bleiben.
Eine spätere Host-Auswahl erfordert dann keine Änderung der Ressourcenidentität.

Ein doco-cd kann bereits mehrere Docker Contexts bedienen. Ebenso können später
mehrere unabhängige doco-cd-Instanzen angebunden werden. Für Containerdaten muss
das Backend jeweils auch die passende Docker Engine erreichen. Authentifizierte
Remote-Verbindungen über SSH oder TLS gehören in einen späteren Ausbauschritt.

## Umsetzungsschritte und Abnahmekriterien

1. **Grundgerüst:** Go-Server, React-Frontend, Vite, Tailwind, shadcn/ui, Query und
   Router einrichten. Der Produktionsbuild wird als ein Container ausgeliefert;
   direkt aufgerufene Frontend-Routen funktionieren.
2. **Inventar:** lokales Ziel, Docker- und doco-cd-Adapter, Dashboard sowie
   Container-, Projekt- und Netzwerkansichten anbinden. Zugehörigkeiten und
   getrennte Verbindungsfehler werden mit echten API-Daten korrekt angezeigt.
3. **Betrieb:** Logs und unterstützte Container-/Compose-Aktionen ergänzen.
   Bei zwei konfigurierten GitOps-Quellen startet das Dashboard den Abgleich für
   die ausgewählte Quelle und zeigt den zugehörigen Laufstatus einschließlich
   übersprungener Läufe. Laufende, erfolgreiche und fehlgeschlagene Aktionen
   bleiben unterscheidbar.
   Nicht unterstützte Aktionen sind nicht ausführbar.
4. **Statistiken:** Container-CPU, RAM und kurzen Verlauf integrieren.
   Werte sind nachvollziehbar beschriftet und die Erfassung bleibt begrenzt.
5. **Auslieferung:** Compose-Beispiel neben doco-cd, Konfigurationsanleitung und
   dokumentierte Versionsanforderungen vervollständigen.

## Noch zu konkretisieren

- Mindestversion von doco-cd, Erkennung verwalteter Ressourcen und tatsächlich
  verfügbare Detailinformationen und Aktionen.
- Zugang zur Control UI: eigene Sitzung oder Einbindung einer vorhandenen
  Authentifizierung. Dies wird vor einem nutzbaren schreibenden Zugriff festgelegt.
- Oberflächensprache sowie visuelle Gestaltung für helle und dunkle Darstellung.
- Messintervall und Grenzen des kurzen Statistikverlaufs; zusätzliche Host-Metriken.

Mehrere Hosts, langfristige Metrikspeicherung, ein Compose-Editor und Löschaktionen
mit Datenbereinigung werden nach dem ersten Umfang geplant.

## Technische Referenzen

- [doco-cd REST-API](https://doco.cd/latest/Endpoints/REST-API/)
- [doco-cd Docker Contexts](https://doco.cd/latest/Advanced/Docker-Contexts/)
- [Docker Engine SDKs](https://docs.docker.com/reference/api/engine/sdk/)
- [Docker Compose-Projektnamen](https://docs.docker.com/compose/how-tos/project-name/)
- [shadcn/ui mit Vite, React, TypeScript und Tailwind](https://ui.shadcn.com/docs/installation/vite)
- [TanStack Query](https://tanstack.com/query/latest/docs/framework/react/overview)
- [TanStack Router](https://tanstack.com/router/latest/docs/overview)
- [shadcn/ui Data Table](https://ui.shadcn.com/docs/components/data-table)
- [Go embed](https://pkg.go.dev/embed)
