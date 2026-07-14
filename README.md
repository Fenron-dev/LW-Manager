# VaultApp

Portable Desktop-Anwendung zur Katalogisierung externer Datenträger. Der aktuelle Stand umfasst die Wails-Oberfläche, sichere Vault-Pfadlogik, einen rekursiven Metadaten-Scanner einschließlich Bildabmessungen, den portablen SQLite-Katalog, eine durchsuchbare Bibliothek und vollständig cloudbasierte Builds.

Bei einem erneuten Scan ersetzt VaultApp den aktiven Katalog vollständig durch den aktuellen Inhalt der Quelle. Der vorherige Stand wird als Archivstand gespeichert und erscheint nicht in der normalen Bibliothek. Wichtige Stände lassen sich gegen manuelles Löschen und die automatische Archivbereinigung schützen.

Der Tab **Archiv** vergleicht den aktuellen Inhalt mit einem wählbaren früheren Stand und markiert neue, entfernte, geänderte und unveränderte Pfade farblich. In der Bibliothek kann eine optionale Duplikatprüfung gestartet werden. Sie bildet zunächst Größenkandidaten und liest nur diese Dateien für einen SHA-256-Inhaltsvergleich.

## Builds ohne lokale Toolchain

1. Repository zu GitHub pushen.
2. Unter **Actions → Build portable apps → Run workflow** einen Build starten.
3. Die vier Pakete nach Abschluss unter **Artifacts** herunterladen.

Ein Tag wie `v0.1.0` baut dieselben Pakete und veröffentlicht sie zusätzlich als GitHub Release. Go, Wails, GTK und weitere Build-Abhängigkeiten werden ausschließlich auf GitHub-Runnern installiert.

## Portable Struktur

Beim ersten Start werden relativ zur `.vaultapp`-Markierung `data/` und `assets/` angelegt. Mit `VAULT_ROOT` kann für Entwicklung und Diagnose ein anderer Stammordner gewählt werden. GGUF-Modelle werden bewusst nicht im Repository oder Release gespeichert.

Die Einstellungen werden portabel unter `data/config.json` abgelegt. Dort lassen sich die Archivierung früherer Scans, die Anzahl aufbewahrter Archivstände sowie die Größenlimits für Bild-, PDF- und Video-Vorschauen steuern. Die Bildanalyse für JPEG, PNG, GIF und HEIC/HEIF kann insgesamt und je Format geschaltet werden. Für das Lesen der Bild-Header gelten ein Limit pro Datei und ein Gesamtbudget pro Scan; beide können wahlweise unbegrenzt sein. Bildvorschauen besitzen entsprechend ein Quelldatei- und ein Gesamtlimit für den portablen Thumbnail-Cache. Auf macOS erzeugt der vorhandene Systemdecoder auch HEIC/HEIF-Vorschauen; diese lassen sich separat deaktivieren. Optional erfasst der Scan außerdem Kamera, Aufnahmedatum, Objektiv und Orientierung aus JPEG-EXIF-Daten; GPS-Informationen werden bewusst nicht gespeichert. Auch dafür gelten ein Datei- und ein Gesamtbudget mit Unbegrenzt-Schaltern.

Der optionale Volltextindex erfasst freigegebene UTF-8-Dokumente, strukturierte Datendateien und Quellcode. Dateilimit, Gesamtbudget und die drei Formatgruppen werden in den Einstellungen gesteuert. In der Bibliothek entscheidet der Schalter **Auch indexierte Dateiinhalte durchsuchen**, ob eine Suche zusätzlich den gespeicherten Inhalt berücksichtigt.

Im Tab **Datenträger** erkennt VaultApp angeschlossene externe Volumes automatisch. Bereits katalogisierte Medien werden über ihre Volume-ID beziehungsweise ihren Einbindungspfad zugeordnet und können direkt erneut gescannt werden. Die Erkennung lässt sich in den Einstellungen abschalten und belegt selbst keinen Cache- oder Katalogspeicher; der manuelle Ordnerdialog bleibt unabhängig davon verfügbar.

Datenträger und einzelne Archivstände können jeweils eine Bemerkung und mehrere frei vergebene Tags erhalten. Tags werden zentral und ohne Beachtung der Groß-/Kleinschreibung verwaltet. Die Datenträgerliste lässt sich nach ihnen filtern; in der Bibliothek beschränkt ein Tag die Treffer auf Dateien entsprechend markierter Datenträger. Im Archiv filtert er die auswählbaren Scan-Stände. Geschützte Archivstände werden weder über den Löschknopf noch durch das eingestellte automatische Aufbewahrungslimit entfernt; deshalb darf ihre Anzahl das Limit überschreiten.

Unter **Einstellungen → Tag-Verwaltung** lassen sich Tags global umbenennen, zusammenführen und löschen. Beim Zusammenführen bleiben alle Zuordnungen zu Datenträgern und Archivständen erhalten; das Löschen entfernt den Tag dagegen aus allen Zuordnungen.

Unter **Einstellungen → Datensicherung** lässt sich ein portables ZIP-Backup erstellen. Es enthält einen konsistenten SQLite-Schnappschuss, `config.json`, ein Formatmanifest und auf Wunsch den Thumbnail-Cache. Die Originaldateien der katalogisierten Datenträger und lokale Modelle werden nicht kopiert. Die Funktion ist abschaltbar; für die ZIP-Größe gilt ein konfigurierbares Gesamtlimit mit optionalem Unbegrenzt-Modus.

Vor einer Wiederherstellung prüft VaultApp das ZIP vollständig: Format und Version, sichere relative Pfade, Größenlimits, ZIP-Prüfsummen, Konfiguration und SQLite-Integrität. Erst danach wird die Wiederherstellung freigegeben. Unmittelbar vor dem Austausch entsteht im portablen Vault eine datierte `VaultApp-Rollback-…zip`; schlägt der Austausch fehl, werden die bisherigen Dateien direkt zurückgesetzt. Enthält das gewählte Backup keine Vorschaubilder, bleibt der vorhandene Thumbnail-Cache unverändert.

Unter macOS muss `VaultApp.app` im heruntergeladenen Paket bleiben: Der portable Vault-Ordner ist der Ordner direkt neben dem App-Bundle. Die Anwendung darf nicht einzeln nach `/Applications` verschoben werden, wenn Daten weiterhin auf dem externen Medium liegen sollen.

Im macOS-Paket liegt außerdem `VaultApp-starten.command`. Die Datei arbeitet relativ zu ihrem eigenen Ordner, entfernt das Quarantäne-Attribut von der danebenliegenden `VaultApp.app`, setzt das interne Programm auf ausführbar und startet die App. App und Starterdatei müssen daher im selben Ordner bleiben.

## Roadmap

- optionale lokale/entfernte KI-Anbieter

Das vollständige Ausgangskonzept liegt unter `Konzepte/VaultApp_Konzept.md`.
