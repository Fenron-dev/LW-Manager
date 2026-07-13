# VaultApp

Portable Desktop-Anwendung zur Katalogisierung externer Datenträger. Der aktuelle Stand umfasst die Wails-Oberfläche, sichere Vault-Pfadlogik, einen rekursiven Metadaten-Scanner einschließlich Bildabmessungen, den portablen SQLite-Katalog, eine durchsuchbare Bibliothek und vollständig cloudbasierte Builds.

Bei einem erneuten Scan ersetzt VaultApp den aktiven Katalog vollständig durch den aktuellen Inhalt der Quelle. Der vorherige Stand wird als löschbarer Archivstand gespeichert und erscheint nicht in der normalen Bibliothek.

Der Tab **Archiv** vergleicht den aktuellen Inhalt mit einem wählbaren früheren Stand und markiert neue, entfernte, geänderte und unveränderte Pfade farblich. In der Bibliothek kann eine optionale Duplikatprüfung gestartet werden. Sie bildet zunächst Größenkandidaten und liest nur diese Dateien für einen SHA-256-Inhaltsvergleich.

## Builds ohne lokale Toolchain

1. Repository zu GitHub pushen.
2. Unter **Actions → Build portable apps → Run workflow** einen Build starten.
3. Die vier Pakete nach Abschluss unter **Artifacts** herunterladen.

Ein Tag wie `v0.1.0` baut dieselben Pakete und veröffentlicht sie zusätzlich als GitHub Release. Go, Wails, GTK und weitere Build-Abhängigkeiten werden ausschließlich auf GitHub-Runnern installiert.

## Portable Struktur

Beim ersten Start werden relativ zur `.vaultapp`-Markierung `data/` und `assets/` angelegt. Mit `VAULT_ROOT` kann für Entwicklung und Diagnose ein anderer Stammordner gewählt werden. GGUF-Modelle werden bewusst nicht im Repository oder Release gespeichert.

Die Einstellungen werden portabel unter `data/config.json` abgelegt. Dort lassen sich die Archivierung früherer Scans, die Anzahl aufbewahrter Archivstände sowie die Größenlimits für Bild-, PDF- und Video-Vorschauen steuern. Die Bildanalyse für JPEG, PNG und GIF kann insgesamt und je Format geschaltet werden. Für das Lesen der Bild-Header gelten ein Limit pro Datei und ein Gesamtbudget pro Scan; beide können wahlweise unbegrenzt sein. Bildvorschauen besitzen entsprechend ein Quelldatei- und ein Gesamtlimit für den portablen Thumbnail-Cache. Optional erfasst der Scan außerdem Kamera, Aufnahmedatum, Objektiv und Orientierung aus JPEG-EXIF-Daten; GPS-Informationen werden bewusst nicht gespeichert. Auch dafür gelten ein Datei- und ein Gesamtbudget mit Unbegrenzt-Schaltern.

Der optionale Volltextindex erfasst freigegebene UTF-8-Dokumente, strukturierte Datendateien und Quellcode. Dateilimit, Gesamtbudget und die drei Formatgruppen werden in den Einstellungen gesteuert. In der Bibliothek entscheidet der Schalter **Auch indexierte Dateiinhalte durchsuchen**, ob eine Suche zusätzlich den gespeicherten Inhalt berücksichtigt.

Im Tab **Datenträger** erkennt VaultApp angeschlossene externe Volumes automatisch. Bereits katalogisierte Medien werden über ihre Volume-ID beziehungsweise ihren Einbindungspfad zugeordnet und können direkt erneut gescannt werden. Die Erkennung lässt sich in den Einstellungen abschalten und belegt selbst keinen Cache- oder Katalogspeicher; der manuelle Ordnerdialog bleibt unabhängig davon verfügbar.

Unter macOS muss `VaultApp.app` im heruntergeladenen Paket bleiben: Der portable Vault-Ordner ist der Ordner direkt neben dem App-Bundle. Die Anwendung darf nicht einzeln nach `/Applications` verschoben werden, wenn Daten weiterhin auf dem externen Medium liegen sollen.

Im macOS-Paket liegt außerdem `VaultApp-starten.command`. Die Datei arbeitet relativ zu ihrem eigenen Ordner, entfernt das Quarantäne-Attribut von der danebenliegenden `VaultApp.app`, setzt das interne Programm auf ausführbar und startet die App. App und Starterdatei müssen daher im selben Ordner bleiben.

## Roadmap

- weitere Vorschauformate (HEIC)
- optionale lokale/entfernte KI-Anbieter

Das vollständige Ausgangskonzept liegt unter `Konzepte/VaultApp_Konzept.md`.
