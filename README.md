<img src="assets/icon.svg" width="64" height="64" alt="">

# doco-ui

Geplante, schlanke Control UI für doco-cd und den zugehörigen Docker-Host.

Die Oberfläche soll Container, Compose-Projekte, Netzwerke und Deployments
sichtbar und bedienbar machen. Der Einstieg ist ein lokaler Host; das Datenmodell
berücksichtigt spätere weitere Ziele.

## Geplanter Stack

- Frontend: React, TypeScript, Tailwind CSS, shadcn/ui und TanStack Query.
- Ergänzender Vorschlag: Vite, TanStack Router und TanStack Table für Ressourcenlisten.
- Backend: Go mit Anbindung an die doco-cd- und Docker-Engine-APIs.
- Betrieb: eigener Container neben doco-cd; Go liefert das gebaute Frontend mit aus.

## Stand

Das Projekt befindet sich in der Planung. Anwendung und Container-Build sind noch
nicht implementiert.

Der [Projektplan](docs/plan.md) hält Umfang, Architektur, offene Entscheidungen
und die vorgesehenen Umsetzungsschritte fest.
