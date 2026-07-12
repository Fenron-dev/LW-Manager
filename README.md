# VaultApp

Portable Desktop-Anwendung zur Katalogisierung externer Datenträger. Der aktuelle Stand ist das technische Fundament: Wails-Oberfläche, sichere Vault-Pfadlogik, Datenbankschema und vollständig cloudbasierte Builds.

## Builds ohne lokale Toolchain

1. Repository zu GitHub pushen.
2. Unter **Actions → Build portable apps → Run workflow** einen Build starten.
3. Die vier Pakete nach Abschluss unter **Artifacts** herunterladen.

Ein Tag wie `v0.1.0` baut dieselben Pakete und veröffentlicht sie zusätzlich als GitHub Release. Go, Wails, GTK und weitere Build-Abhängigkeiten werden ausschließlich auf GitHub-Runnern installiert.

## Portable Struktur

Beim ersten Start werden relativ zur `.vaultapp`-Markierung `data/` und `assets/` angelegt. Mit `VAULT_ROOT` kann für Entwicklung und Diagnose ein anderer Stammordner gewählt werden. GGUF-Modelle werden bewusst nicht im Repository oder Release gespeichert.

## Roadmap

- Datenträger erkennen und auswählen
- SQLite-Datenzugriff und Scan-Jobs
- Bibliothek, Suche und Thumbnails
- optionale lokale/entfernte KI-Anbieter

Das vollständige Ausgangskonzept liegt unter `Konzepte/VaultApp_Konzept.md`.
