# VaultApp

Portable Desktop-Anwendung zur Katalogisierung externer Datenträger. Der aktuelle Stand umfasst die Wails-Oberfläche, sichere Vault-Pfadlogik, einen rekursiven Metadaten-Scanner einschließlich Bildabmessungen, den portablen SQLite-Katalog, eine durchsuchbare Bibliothek und vollständig cloudbasierte Builds.

Der abgeschlossene Funktionsstand wird als **VaultApp 1.0.0** für macOS ARM, macOS Intel, Windows x64 und Linux x64 veröffentlicht. Die stabilen Pakete werden ausschließlich durch GitHub Actions erzeugt und gemeinsam mit SHA-256-Prüfsummen im GitHub Release bereitgestellt.

Bei einem erneuten Scan ersetzt VaultApp den aktiven Katalog vollständig durch den aktuellen Inhalt der Quelle. Der vorherige Stand wird als Archivstand gespeichert und erscheint nicht in der normalen Bibliothek. Wichtige Stände lassen sich gegen manuelles Löschen und die automatische Archivbereinigung schützen.

Der Tab **Archiv** vergleicht den aktuellen Inhalt mit einem wählbaren früheren Stand und markiert neue, entfernte, geänderte und unveränderte Pfade farblich. In der Bibliothek kann eine optionale Duplikatprüfung gestartet werden. Sie bildet zunächst Größenkandidaten und liest nur diese Dateien für einen SHA-256-Inhaltsvergleich.

Die Duplikatprüfung kann in den Einstellungen vollständig deaktiviert werden. Ein Limit pro Kandidat und ein Gesamtbudget pro Prüflauf begrenzen das vom Datenträger gelesene Datenvolumen; beide Grenzen lassen sich unabhängig auf unbegrenzt setzen. Bereits vorhandene Prüfsummen werden erneut verwendet, solange der katalogisierte Dateistand unverändert bleibt. Die Originaldateien werden dabei niemals in den Vault kopiert.

Ab Version 0.42.0-dev lassen sich Fundorte einer Duplikatgruppe direkt im Finder, Explorer oder Dateimanager anzeigen. Pro Gruppe kann ein bevorzugtes Original dauerhaft markiert werden; diese Zuordnung bleibt über erneute Scans hinweg anhand von Datenträger und relativem Pfad erhalten. Weitere Kandidaten können gefahrlos vorgemerkt werden, wobei VaultApp Anzahl und mögliche Speicherersparnis berechnet. Die Vormerkung allein verändert oder löscht keine Datei.

Version 0.43.0-dev ergänzt eine wiederherstellbare Duplikat-Quarantäne. Vor jeder Verschiebung werden Pfadgrenzen, regulärer Dateityp, Größe, Änderungszeit und SHA-256 erneut geprüft; bevorzugte Originale, veränderte Dateien, symbolische Links und offline liegende Datenträger werden abgewiesen. Die Quarantäne ersetzt keine vorhandene Datei und lässt sich in den Einstellungen aktivieren sowie pro Datei und insgesamt begrenzen. Beide Grenzen besitzen einen Unbegrenzt-Schalter. Wiederhergestellte Dateien erscheinen nach einem erneuten Scan wieder im aktuellen Katalog.

Das dauerhafte Löschen aus der Quarantäne ist ab Version 0.44.0-dev verfügbar und standardmäßig deaktiviert. Nach der Freischaltung in den Einstellungen muss für jede Datei zusätzlich exakt `DAUERHAFT LÖSCHEN` eingegeben werden. Unmittelbar vor dem Entfernen prüft VaultApp erneut, dass die Datei innerhalb des Quarantäneverzeichnisses liegt, kein symbolischer Link ist und weiterhin der gespeicherten Größe und SHA-256-Prüfsumme entspricht. Die Funktion gibt Speicher frei und erzeugt keine dauerhaften Daten; deshalb besitzt sie kein Speicherlimit.

## Builds ohne lokale Toolchain

1. Repository zu GitHub pushen.
2. Unter **Actions → Build portable apps → Run workflow** einen Build starten.
3. Die vier Pakete nach Abschluss unter **Artifacts** herunterladen.

Diese Entwicklungsbuilds tragen die im Quellcode hinterlegte `-dev`-Version, liegen nur als zeitlich begrenzte Workflow-Artefakte vor und erzeugen kein GitHub Release. Go, Wails, GTK und weitere Build-Abhängigkeiten werden ausschließlich auf GitHub-Runnern installiert.

Ein stabiles Release wird ab Version 0.50.0-dev ausschließlich bewusst unter **Actions → Publish stable release → Run workflow** gestartet. Dort müssen die stabile SemVer-Version ohne `v` und zusätzlich exakt `RELEASE` eingegeben werden. Der Workflow akzeptiert nur die zur aktuellen Entwicklungsversion passende Nummer, prüft Tests und Formatierung und legt anschließend den annotierten Tag an. Dieser startet die plattformübergreifenden Builds und veröffentlicht dauerhaft macOS-ARM-, macOS-Intel- und Linux-Pakete als `tar.gz`, Windows als `zip` sowie eine gemeinsame `SHA256SUMS.txt`. Die stabile Versionsnummer wird beim Build in alle Programme eingebettet. Bereits vorhandene Tags werden abgewiesen.

## Portable Struktur

Beim ersten Start werden relativ zur `.vaultapp`-Markierung `data/` und `assets/` angelegt. Mit `VAULT_ROOT` kann für Entwicklung und Diagnose ein anderer Stammordner gewählt werden. GGUF-Modelle werden bewusst nicht im Repository oder Release gespeichert.

Die Einstellungen werden portabel unter `data/config.json` abgelegt. Dort lassen sich die Archivierung früherer Scans, die Anzahl aufbewahrter Archivstände sowie die Größenlimits für Bild-, PDF- und Video-Vorschauen steuern. Die Bildanalyse für JPEG, PNG, GIF und HEIC/HEIF kann insgesamt und je Format geschaltet werden. Für das Lesen der Bild-Header gelten ein Limit pro Datei und ein Gesamtbudget pro Scan; beide können wahlweise unbegrenzt sein. Bildvorschauen besitzen entsprechend ein Quelldatei- und ein Gesamtlimit für den portablen Thumbnail-Cache. Auf macOS erzeugt der vorhandene Systemdecoder auch HEIC/HEIF-Vorschauen; diese lassen sich separat deaktivieren. Optional erfasst der Scan außerdem Kamera, Aufnahmedatum, Objektiv und Orientierung aus JPEG-EXIF-Daten; GPS-Informationen werden bewusst nicht gespeichert. Auch dafür gelten ein Datei- und ein Gesamtbudget mit Unbegrenzt-Schaltern.

PDF- und Videovorschauen lassen sich unabhängig aktivieren und besitzen jeweils ein Quelldateilimit mit optionalem Unbegrenzt-Modus. Sie werden ausschließlich für die gerade geöffnete Detailansicht in den Arbeitsspeicher geladen und nicht im Vault zwischengespeichert, weshalb kein dauerhafter Gesamt-Speicherverbrauch entsteht. Vor jeder Vorschau prüft VaultApp außerdem, ob es weiterhin dieselbe reguläre Datei wie beim letzten Scan ist; nachträglich ausgetauschte Dateien oder symbolische Verknüpfungen werden abgewiesen.

Die Dateidetails zeigen, ob die katalogisierte Originaldatei noch erreichbar und seit dem Scan unverändert ist. Von dort kann sie im Finder, Explorer oder Linux-Dateimanager angezeigt, ihr Ordner geöffnet oder ihr relativer beziehungsweise vollständiger Pfad kopiert werden. Vor externen Dateiaktionen prüft VaultApp regulären Dateityp, Größe, Änderungszeit und aufgelöste Verknüpfungen erneut. Nicht angeschlossene Datenträger werden in der Datenträgerliste als offline markiert; reine Katalog- und Archivansichten bleiben weiterhin verfügbar.

Der optionale Volltextindex erfasst freigegebene UTF-8-Dokumente, strukturierte Datendateien, Quellcode und ab Version 0.45.0-dev auch Text aus PDF-Dateien. PDF-Text wird vollständig lokal und ohne externe Programme aus unkomprimierten oder Flate-komprimierten Textobjekten gelesen. Version 0.46.0-dev ergänzt DOCX und ODT als separat aktivierbare Office-Gruppe; Version 0.47.0-dev erweitert sie um XLSX und ODS. Version 0.48.0-dev schließt die Office-Grundformate mit PPTX und ODP ab und erfasst dabei Folientexte sowie vorhandene PPTX-Sprechernotizen. Bei Tabellen werden gemeinsame Zeichenketten, Inline-Texte und gespeicherte Zellwerte erfasst. Formeln, Makros, Animationen, Medien, eingebettete Objekte und externe Konverter werden nicht ausgeführt. Verschlüsselte Dokumente und reine Bild-/Scan-Dokumente liefern bewusst keinen Text; dafür ist später das getrennt aktivierbare OCR-Modul vorgesehen.

Für den Volltextindex gelten drei unabhängige Grenzen: Rohdaten pro Datei, gelesene Rohdaten pro Scan und dauerhaft gespeicherter Indextext im gesamten Katalog. Alle Grenzen besitzen einen Unbegrenzt-Schalter. Beim erneuten Scan wird der bisherige Index des betroffenen Datenträgers aus der Speicherberechnung herausgenommen und anschließend durch dessen aktuellen Stand ersetzt. In der Bibliothek entscheidet der Schalter **Auch indexierte Dateiinhalte durchsuchen**, ob eine Suche zusätzlich den gespeicherten Inhalt berücksichtigt.

Ab Version 0.49.0-dev lassen sich unter **Einstellungen → Scanprofile** wiederverwendbare Kombinationen aus Ausschlussregeln, Bild-/EXIF-Analyse und Volltextindex anlegen. Im Bearbeiten-Dialog eines Datenträgers wird das Profil zugeordnet und beim nächsten Scan automatisch angewendet. Das tatsächlich verwendete Profil erscheint vor und während des Scans sowie im Diagnosebericht. Sämtliche Datei- und Gesamtbudgets bleiben globale Obergrenzen einschließlich ihrer Unbegrenzt-Schalter; ein Profil kann diese Grenzen daher nicht unbemerkt erhöhen. Wird ein zugeordnetes Profil später gelöscht, fällt der Datenträger sicher auf die globalen Einstellungen zurück.

Die aktuell gefilterte Bibliotheksansicht lässt sich als UTF-8-CSV und ab Version 0.51.0-dev zusätzlich als maschinenlesbares JSON exportieren. Enthalten sind Dateiname, Datenträger, relativer Pfad, Typ, Größe, Änderungsdatum, manuelle Tags und vorhandene KI-Metadaten; im JSON werden außerdem Formatversion, Exportzeitpunkt und die verwendeten Filter festgehalten. Originaldateien werden nicht kopiert. Beide Exporte werden fortlaufend geschrieben und landen nur am ausdrücklich gewählten Ziel; die CSV schützt Tabellenprogramme zusätzlich vor Formelausführung durch präparierte Textfelder. Unter **Einstellungen → Exporte und Berichte** lassen sich beide Formate getrennt deaktivieren und ihre jeweilige maximale Gesamtgröße begrenzen oder auf unbegrenzt setzen.

Ab Version 0.52.0-dev kann der aktuell gefilterte Archivvergleich ebenfalls als maschinenlesbarer JSON-Bericht exportiert werden. Der Bericht enthält Datenträger und Archivstand einschließlich Tags, Schutzstatus und Bemerkung, die verwendeten Status- und Pfadfilter sowie beide Seiten jedes Vergleichseintrags. Die Ausgabe wird fortlaufend geschrieben und besitzt unter **Einstellungen → Exporte und Berichte** eine eigene Aktivierung und ein eigenes Gesamtlimit mit Unbegrenzt-Schalter.

Version 0.53.0-dev ergänzt eine eigenständige HTML-Druckansicht für denselben gefilterten Vergleich. Sie enthält eine Zusammenfassung nach Neu, Entfernt, Geändert und Unverändert sowie eine tabellarische Gegenüberstellung. Die farbigen Statusmarkierungen umfassen die vollständige Zeile und bleiben beim Drucken oder Speichern als PDF erhalten. Die Datei benötigt keine Netzwerkverbindung und öffnet sich in jedem aktuellen Browser. Aktivierung und Gesamtlimit werden unabhängig vom JSON-Bericht gesteuert.

Im Tab **Datenträger** erkennt VaultApp angeschlossene externe Volumes automatisch. Bereits katalogisierte Medien werden über ihre Volume-ID beziehungsweise ihren Einbindungspfad zugeordnet und können direkt erneut gescannt werden. Die Erkennung lässt sich in den Einstellungen abschalten und belegt selbst keinen Cache- oder Katalogspeicher; der manuelle Ordnerdialog bleibt unabhängig davon verfügbar.

Datenträger, einzelne Dateien und Archivstände können mehrere frei vergebene Tags erhalten; Datenträger und Archivstände zusätzlich eine Bemerkung. Tags werden zentral und ohne Beachtung der Groß-/Kleinschreibung verwaltet. Manuelle Datei-Tags werden in den Dateidetails bearbeitet und bleiben bei erneuten Scans desselben relativen Pfads erhalten. Fehlt der Pfad beim nächsten Scan, entfernt VaultApp seine Zuordnungen automatisch. Die Datenträgerliste lässt sich nach Tags filtern; der Bibliotheksfilter berücksichtigt sowohl direkt markierte Dateien als auch alle Dateien eines markierten Datenträgers. Im Archiv filtert er die auswählbaren Scan-Stände. Geschützte Archivstände werden weder über den Löschknopf noch durch das eingestellte automatische Aufbewahrungslimit entfernt; deshalb darf ihre Anzahl das Limit überschreiten.

Unter **Einstellungen → Tag-Verwaltung** lassen sich Tags global umbenennen, zusammenführen und löschen. Die Übersicht zeigt ihre Nutzung auf Datenträgern, Dateien und Archivständen. Beim Zusammenführen bleiben sämtliche Zuordnungen erhalten; das Löschen entfernt den Tag dagegen überall.

Jeder Scan kann einen portablen JSON-Diagnosebericht unter `data/logs` anlegen. Die Berichte zeigen Laufzeit, Dateizahl, übersprungene Pfade und konkrete Lesefehler und sind unter **Einstellungen → Scan-Diagnose** einsehbar. Die Funktion sowie das Limit pro Bericht und für alle Scan-Logs lassen sich dort ein- oder ausschalten beziehungsweise auf unbegrenzt setzen. Ist ein Datenträger bereits beim Scanstart nicht mehr erreichbar, wird der Scan abgebrochen und der bisherige Katalogstand nicht durch einen leeren Stand ersetzt.

Unter **Einstellungen → Scan-Ausschlüsse** lassen sich Systemreste, typische Entwicklungsordner und bis zu 100 eigene Datei- oder Pfadmuster vom Scan ausnehmen. Die Regeln sind standardmäßig deaktiviert, damit bestehende Scans unverändert bleiben. Einfache Namen gelten in jeder Verzeichnisebene; Muster mit `/` beziehen sich auf den relativen Pfad und unterstützen `*` sowie `?`. Ausgeschlossene Inhalte werden weder gelesen noch im aktuellen Katalog gespeichert. Die Diagnose weist ihre Anzahl getrennt von echten Lesefehlern aus; bei aktivem Scan-Archiv bleibt der vorherige vollständige Stand erhalten.

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

### Nächste Umsetzungsschritte

1. **Dateikomfort und Offline-Status – umgesetzt in 0.41.0-dev**
   - Datei beziehungsweise übergeordneten Ordner im Finder, Explorer oder Dateimanager anzeigen.
   - Relativen und vollständigen Pfad kopieren.
   - Nicht angeschlossene Datenträger eindeutig als offline kennzeichnen und Dateiaktionen dann deaktivieren.

2. **Sichere Duplikatverwaltung – umgesetzt in 0.44.0-dev**
   - Fundorte direkt aus einer Duplikatgruppe öffnen und ein bevorzugtes Original markieren – umgesetzt.
   - Kandidaten einzeln auswählen und Größe der möglichen Einsparung anzeigen – umgesetzt.
   - Entfernen ausschließlich nach erneuter Prüfung von Größe, Änderungszeit und SHA-256 sowie ausdrücklicher Bestätigung; symbolische Links und nicht erreichbare Datenträger werden abgewiesen – umgesetzt.
   - Wiederherstellbare Quarantäne und getrennt freizuschaltendes dauerhaftes Löschen mit Texteingabe – umgesetzt.

3. **Erweiterte Inhaltsindizierung – PDF und Office-Grundformate umgesetzt in 0.48.0-dev**
   - Lokale PDF-, DOCX-, ODT-, XLSX-, ODS-, PPTX- und ODP-Textextraktion ohne externe Programme – umgesetzt; optionales OCR für Bilder und Scans bleibt offen.
   - Jede Formatgruppe separat aktivierbar machen – für Text, Daten, Quellcode, PDF und Office umgesetzt.
   - Lesegrenzen pro Datei und pro Scan sowie ein Gesamtlimit für dauerhaft gespeicherten Indextext jeweils mit Unbegrenzt-Schalter anbieten – umgesetzt.
   - Extraktion lokal ausführen; externe KI-Dienste nur nach ausdrücklicher Freigabe verwenden – für alle unterstützten Formate umgesetzt.

4. **Scan-Profile pro Datenträger – umgesetzt in 0.49.0-dev**
   - Globale Scan-Einstellungen optional je Datenträger überschreiben – umgesetzt.
   - Ausschlussmuster, Inhaltsindex und Bild-/EXIF-Analyse in wiederverwendbaren Profilen bündeln – umgesetzt.
   - Vor jedem Scan das tatsächlich verwendete Profil anzeigen und in der Diagnose festhalten – umgesetzt.

5. **Versionierte GitHub-Releases – umgesetzt, erste stabile Veröffentlichung 1.0.0**
   - Freigegebene Versionen automatisch als GitHub-Release veröffentlichen – umgesetzt.
   - Portable Pakete für macOS ARM/Intel, Windows und Linux dauerhaft anhängen und mit SHA-256-Prüfsummen versehen – umgesetzt.
   - Entwicklungsbuilds und stabile Releases klar trennen; ein Release bleibt eine ausdrücklich gestartete Aktion – umgesetzt.

6. **Erweiterte Exporte und Berichte – umgesetzt in 0.53.0-dev**
   - Gefilterten Katalog zusätzlich als JSON exportieren – umgesetzt.
   - Archivvergleiche als maschinenlesbaren Bericht und druckbare Änderungsübersicht exportieren – umgesetzt.
   - Exportarten einzeln aktivierbar machen und ihre maximale Gesamtgröße jeweils mit Unbegrenzt-Schalter steuern – umgesetzt.

Für neue Funktionen gilt weiterhin: Aktivierung und Ressourcenlimits werden unter **Einstellungen** angeboten, wenn Dateien gelesen oder Daten dauerhaft im Vault gespeichert werden. Nicht verändernde Funktionen ohne eigenen Speicherverbrauch benötigen kein künstliches Speicherlimit.

Das vollständige Ausgangskonzept liegt unter `Konzepte/VaultApp_Konzept.md`.
