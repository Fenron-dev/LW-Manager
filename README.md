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

Die aktuell gefilterte Bibliotheksansicht lässt sich als UTF-8-CSV exportieren. Enthalten sind Dateiname, Datenträger, relativer Pfad, Typ, Größe, Änderungsdatum, manuelle Tags und vorhandene KI-Metadaten; Originaldateien werden nicht kopiert. Der Export wird zeilenweise geschrieben, schützt Tabellenprogramme vor Formelausführung durch präparierte Textfelder und landet nur am ausdrücklich gewählten Ziel. Unter **Einstellungen → Katalogexport** lässt sich die Funktion deaktivieren und die maximale CSV-Gesamtgröße begrenzen oder auf unbegrenzt setzen.

Im Tab **Datenträger** erkennt VaultApp angeschlossene externe Volumes automatisch. Bereits katalogisierte Medien werden über ihre Volume-ID beziehungsweise ihren Einbindungspfad zugeordnet und können direkt erneut gescannt werden. Die Erkennung lässt sich in den Einstellungen abschalten und belegt selbst keinen Cache- oder Katalogspeicher; der manuelle Ordnerdialog bleibt unabhängig davon verfügbar.

Datenträger, einzelne Dateien und Archivstände können mehrere frei vergebene Tags erhalten; Datenträger und Archivstände zusätzlich eine Bemerkung. Tags werden zentral und ohne Beachtung der Groß-/Kleinschreibung verwaltet. Manuelle Datei-Tags werden in den Dateidetails bearbeitet und bleiben bei erneuten Scans desselben relativen Pfads erhalten. Fehlt der Pfad beim nächsten Scan, entfernt VaultApp seine Zuordnungen automatisch. Die Datenträgerliste lässt sich nach Tags filtern; der Bibliotheksfilter berücksichtigt sowohl direkt markierte Dateien als auch alle Dateien eines markierten Datenträgers. Im Archiv filtert er die auswählbaren Scan-Stände. Geschützte Archivstände werden weder über den Löschknopf noch durch das eingestellte automatische Aufbewahrungslimit entfernt; deshalb darf ihre Anzahl das Limit überschreiten.

Unter **Einstellungen → Tag-Verwaltung** lassen sich Tags global umbenennen, zusammenführen und löschen. Die Übersicht zeigt ihre Nutzung auf Datenträgern, Dateien und Archivständen. Beim Zusammenführen bleiben sämtliche Zuordnungen erhalten; das Löschen entfernt den Tag dagegen überall.

Jeder Scan kann einen portablen JSON-Diagnosebericht unter `data/logs` anlegen. Die Berichte zeigen Laufzeit, Dateizahl, übersprungene Pfade und konkrete Lesefehler und sind unter **Einstellungen → Scan-Diagnose** einsehbar. Die Funktion sowie das Limit pro Bericht und für alle Scan-Logs lassen sich dort ein- oder ausschalten beziehungsweise auf unbegrenzt setzen. Ist ein Datenträger bereits beim Scanstart nicht mehr erreichbar, wird der Scan abgebrochen und der bisherige Katalogstand nicht durch einen leeren Stand ersetzt.

Unter **Einstellungen → Datensicherung** lässt sich ein portables ZIP-Backup erstellen. Es enthält einen konsistenten SQLite-Schnappschuss, `config.json`, ein Formatmanifest und auf Wunsch den Thumbnail-Cache. Die Originaldateien der katalogisierten Datenträger und lokale Modelle werden nicht kopiert. Die Funktion ist abschaltbar; für die ZIP-Größe gilt ein konfigurierbares Gesamtlimit mit optionalem Unbegrenzt-Modus.

Vor einer Wiederherstellung prüft VaultApp das ZIP vollständig: Format und Version, sichere relative Pfade, Größenlimits, ZIP-Prüfsummen, Konfiguration und SQLite-Integrität. Erst danach wird die Wiederherstellung freigegeben. Unmittelbar vor dem Austausch entsteht im portablen Vault eine datierte `VaultApp-Rollback-…zip`; schlägt der Austausch fehl, werden die bisherigen Dateien direkt zurückgesetzt. Enthält das gewählte Backup keine Vorschaubilder, bleibt der vorhandene Thumbnail-Cache unverändert.

Sicherungen und Rückfallsicherungen, die direkt im Vault-Ordner liegen, erscheinen in der Backup-Verwaltung mit Typ, Datum, Größe und gemeinsamem Speicherverbrauch. Sie können dort geprüft, zur Wiederherstellung ausgewählt oder nach Rückfrage gelöscht werden. Löschvorgänge sind technisch auf eindeutig benannte VaultApp-ZIP-Dateien direkt im Vault-Ordner begrenzt; externe Sicherungen können weiterhin über den Dateidialog geprüft werden.

Unter **Einstellungen → KI-Anbieter** kann die optionale KI-Grundlage vollständig deaktiviert oder für ein lokales Ollama beziehungsweise den entfernten Dienst OpenRouter konfiguriert werden. Endpunkt, Modell, Zeitlimit sowie Datenbudgets pro Datei und insgesamt sind einstellbar; beide Budgets besitzen einen Unbegrenzt-Schalter. Der Verbindungstest liest ausschließlich die Modellliste und startet keine Analyse. OpenRouter-Schlüssel werden getrennt unter `data/secrets/ai-provider.key` gespeichert und bewusst nicht in VaultApp-Backups aufgenommen. Auf einem unverschlüsselten portablen Medium bleibt diese Datei dennoch lokal lesbar.

In den Dateidetails kann eine einzelne Datei ausdrücklich zur KI-Analyse gesendet werden. VaultApp überträgt Dateiname, Pfad, Typ, Größe, vorhandene Metadaten und – sofern zuvor über den Volltextindex erfasst – begrenzten Textinhalt. Originaldateien werden für die KI nicht zusätzlich geöffnet. Zusammenfassung, Schlagwörter, Anbieter, Modell und verwendete Textmenge werden im Katalog gespeichert; unveränderte Dateien behalten das Ergebnis bei einem erneuten Scan, während veraltete Ergebnisse geänderter oder entfernter Dateien bereinigt werden. Die erweiterte Bibliothekssuche findet auf Wunsch auch KI-Zusammenfassungen und -Schlagwörter.

Die optionale Vision-Analyse ist davon getrennt und standardmäßig deaktiviert. Für JPEG, PNG, GIF, WebP sowie auf macOS HEIC/HEIF erscheint in den Dateidetails ein eigener Button. Erst nach diesem Klick erzeugt VaultApp eine verkleinerte Vorschau und sendet sie an das konfigurierte Vision-Modell. Modell, Quellbildlimit und maximales Bilddatenvolumen je Anfrage lassen sich separat einstellen; beide Grenzen besitzen einen Schalter für „Unbegrenzt“. Bei entfernten Endpunkten bestätigt der Benutzer die Übertragung vor jeder Analyse. Das Analyseergebnis kennzeichnet, ob Bild- oder Textdaten verwendet wurden.

Unter macOS muss `VaultApp.app` im heruntergeladenen Paket bleiben: Der portable Vault-Ordner ist der Ordner direkt neben dem App-Bundle. Die Anwendung darf nicht einzeln nach `/Applications` verschoben werden, wenn Daten weiterhin auf dem externen Medium liegen sollen.

Im macOS-Paket liegt außerdem `VaultApp-starten.command`. Die Datei arbeitet relativ zu ihrem eigenen Ordner, entfernt das Quarantäne-Attribut von der danebenliegenden `VaultApp.app`, setzt das interne Programm auf ausführbar und startet die App. App und Starterdatei müssen daher im selben Ordner bleiben.

## Roadmap

Die im Ausgangskonzept vorgesehenen Kernfunktionen sind umgesetzt. Weitere Erweiterungen werden anhand der Praxistests priorisiert.

Das vollständige Ausgangskonzept liegt unter `Konzepte/VaultApp_Konzept.md`.
