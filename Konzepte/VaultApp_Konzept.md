# VaultApp – Konzeptdokument

**Version:** 1.0  
**Datum:** 2026-07-12  
**Status:** Entwurf für technische Umsetzung

---

## 1. Überblick und Zielsetzung

**Projektname:** VaultApp  
**Typ:** Portable Desktop-Anwendung zur Verwaltung und Katalogisierung von Inhalten externer Datenträger  
**Framework:** Wails (Go + HTML/htmx/CSS)  
**Plattformen:** Windows (x64), macOS (Intel + ARM), Linux (x64)  
**AI-Integration:** Lokale LLM über llama.cpp (eingebettet), optionale Anbindung an Ollama/OpenRouter

### 1.1 Kernfunktionen (Prioritätsreihenfolge)

| Priorität | Feature | Beschreibung |
|-----------|---------|--------------|
| P0 | Datenträger-Erkennung | Automatische Erkennung angeschlossener USB-Sticks/Festplatten |
| P0 | Inhalts-Scan | Rekursives Einlesen aller Dateien (Metadaten, Thumbnails, EXIF) |
| P0 | Vault-Konzept | Relative Pfade, portable Datenbank, eigenständiger Ordner |
| P1 | Lokale AI-Tagging | Integration von llama.cpp für smarte Kategorisierung |
| P1 | Volltextsuche | Durchsuchen aller katalogisierten Inhalte |
| P2 | Bild-Erkennung | Vision-Modelle für Foto-Analyse |
| P2 | Export/Backup | Export der Datenbank und Einstellungen |

---

## 2. Vault-Architektur

### 2.1 Ordnerstruktur

```
VaultApp/
├── vault/
│   ├── bin/
│   │   ├── windows/
│   │   │   └── VaultApp.exe          # Windows-Executable
│   │   ├── macos/
│   │   │   └── VaultApp.app          # macOS App-Bundle
│   │   ├── linux/
│   │   │   └── vaultapp              # Linux-Binary
│   ├── data/
│   │   ├── vault.db                  # SQLite-Datenbank
│   │   ├── config.json               # App-Konfiguration
│   │   ├── logs/                     # Log-Dateien
│   │   └── cache/                    # Temporäre Dateien
│   ├── assets/
│   │   ├── models/                   # LLM-Modelle (.gguf)
│   │   │   └── default-model.gguf    # Standard-LLM (1-3B Parameter)
│   │   ├── thumbnails/               # Generierte Vorschaubilder
│   │   │   └── [sha256-hash].webp
│   │   ├── icons/                    # App-Icons und Symbole
│   │   └── ui/                       # Frontend-Assets (CSS/JS)
│   └── README.md                     # Kurzanleitung
```

### 2.2 Relativer Pfad-Aufbau in Go

```go
// vault.go – Zentrale Pfadauflösung

import (
    "os"
    "path/filepath"
)

// GetVaultRoot gibt den absoluten Pfad zum Vault-Stammverzeichnis zurück
func GetVaultRoot() (string, error) {
    exePath, err := os.Executable()
    if err != nil {
        return "", err
    }

    // Bin-Verzeichnis ist eine Ebene unter dem Vault
    binDir := filepath.Dir(exePath)
    vaultRoot := filepath.Dir(binDir)

    return vaultRoot, nil
}

// GetDataPath löst relative Pfade innerhalb des Vaults auf
func GetDataPath(relativePath string) (string, error) {
    root, err := GetVaultRoot()
    if err != nil {
        return "", err
    }
    return filepath.Join(root, "data", relativePath), nil
}

// GetAssetPath löst relative Pfade für Assets auf
func GetAssetPath(relativePath string) (string, error) {
    root, err := GetVaultRoot()
    if err != nil {
        return "", err
    }
    return filepath.Join(root, "assets", relativePath), nil
}
```

### 2.3 Pfadlogik je nach Plattform und Aufrufart

```
Windows (direkt):
  vault/bin/windows/VaultApp.exe
  → Bin:  vault/bin/windows/
  → Root: vault/

Windows (im PATH):
  C:\Windows\System32\VaultApp.exe
  → Erkennung: exePath enthält "vault" → als Root verwenden
  → Fallback: Aktuelles Arbeitsverzeichnis (cwd)

macOS (App-Bundle):
  vault/bin/macos/VaultApp.app/Contents/MacOS/VaultApp
  → Bin:  VaultApp.app/Contents/MacOS/
  → Root: vault/ (drei Ebenen hoch von MacOS)

macOS (direkt via CLI):
  vault/bin/macos/VaultApp.app/Contents/MacOS/vaultapp
  → Gleiche Logik wie App-Bundle

Linux:
  vault/bin/linux/vaultapp
  → Bin:  vault/bin/linux/
  → Root: vault/

Linux (ohne execute-Flag, noexec):
  → Fallback auf cwd, wenn exe nicht ausführbar
```

---

## 3. Fallstricke und Lösungen (Pitfall Guide)

### 3.1 SQLite auf exFAT – WAL-Problem

**Problem:**  
exFAT unterstützt keine POSIX-Sperren richtig. SQLites Write-Ahead Logging (WAL) benötigt exklusive Datei-Sperren, die auf exFAT fehlschlagen können. Mehrere Prozesse (oder ein abgestürzter vorheriger Prozess) können die WAL-Datei nicht korrekt aufräumen → Datenbank ist "gesperrt".

**Lösung in Go:**

```go
import (
    "database/sql"
    _ "github.com/mutecinfo/go-sqlite3"
)

// OpenVaultDB öffnet die SQLite-DB mit sicheren Einstellungen für exFAT
func OpenVaultDB(dbPath string) (*sql.DB, error) {
    db, err := sql.Open("sqlite3", dbPath+
        "?_journal_mode=WAL"+      // WAL deaktivieren → TRUNCATE nutzen
        "&_busy_timeout=5000"+     // 5 Sekunden Wartezeit bei Sperren
        "&_synchronous=NORMAL"+    // Kompromiss aus Sicherheit und Speed
        "&_txlock=EXCLUSIVE")      // Exklusive Transaktionen
    if err != nil {
        return nil, err
    }

    // PRAGMA beim Start setzen
    _, err = db.Exec("PRAGMA journal_mode=DELETE")
    if err != nil {
        return nil, err
    }

    return db, nil
}
```

**Empfehlung:** `journal_mode=DELETE` statt `WAL`. Bei Absturz kann es zu Datenverlust kommen, aber bei einer Vault-App mit regelmäßigen Scans ist das akzeptabel. Alternativ: Prüfung beim Start, ob eine `.db-journal`-Datei existiert und automatisches Aufräumen.

---

### 3.2 Relative Pfadauflösung – Go Best Practices

**Problem:**  
`os.Executable()` liefert nicht immer den erwarteten Pfad (Symlinks, macOS Bundle, PATH).

**Robuste Lösung:**

```go
import (
    "os"
    "path/filepath"
    "runtime"
)

// ResolveVaultRoot ermittelt den Vault-Stamm robust
func ResolveVaultRoot() (string, error) {
    // 1. Versuche os.Executable()
    if exePath, err := os.Executable(); err == nil {
        vaultRoot := findVaultRootFromExe(exePath)
        if vaultRoot != "" {
            return vaultRoot, nil
        }
    }

    // 2. Fallback: Aktuelles Arbeitsverzeichnis
    cwd, err := os.Getwd()
    if err != nil {
        return "", err
    }

    // Prüfe, ob cwd ein Vault ist
    if isVaultRoot(cwd) {
        return cwd, nil
    }

    // 3. Fallback: Prüfe Umgebungsvariable VAULT_ROOT
    if vaultRoot := os.Getenv("VAULT_ROOT"); vaultRoot != "" {
        return vaultRoot, nil
    }

    return "", fmt.Errorf("Vault-Root nicht gefunden")
}

// findVaultRootFromExe prüft rekursiv aufwärts nach vault-Marker
func findVaultRootFromExe(exePath string) string {
    current := filepath.Dir(exePath)
    for {
        if isVaultRoot(current) {
            return current
        }
        parent := filepath.Dir(current)
        if parent == current {
            break // Wurzel erreicht
        }
        current = parent
    }
    return ""
}

// isVaultRoot prüft, ob das Verzeichnis ein gültiger Vault ist
func isVaultRoot(path string) bool {
    required := []string{"bin", "data", "assets"}
    for _, name := range required {
        if _, err := os.Stat(filepath.Join(path, name)); os.IsNotExist(err) {
            return false
        }
    }
    return true
}
```

---

### 3.3 macOS App-Bundle – Pfad-Korrekturen

**Problem:**  
Bei macOS `.app`-Bundles ist `os.Executable()` der Pfad zum eigentlichen Binary innerhalb des Bundles:

```
vault/bin/macos/VaultApp.app/Contents/MacOS/VaultApp
                                        ^^^^^^^^^^^^^^
                                         Bin-Verzeichnis
```

Drei Ebenen über `MacOS` ist das Bundle-Stammverzeichnis. Zwei Ebenen über `bin` (also über `macos`) ist der Vault-Root.

**Lösung:**

```go
// IsMacOSAppBundle erkennt, ob der exePath ein .app-Bundle ist
func IsMacOSAppBundle(exePath string) bool {
    return strings.HasSuffix(exePath, ".app/Contents/MacOS/"+filepath.Base(exePath))
}

// GetVaultRootMacOS speziell für App-Bundles
func GetVaultRootMacOS(exePath string) string {
    // Vom exe im MacOS-Ordner zum Bundle-Root
    // VaultApp.app/Contents/MacOS/VaultApp
    //                      ^^^^^^^^
    //                      Contents
    //              ^^^^^^^^^^^^^^^^
    //              Contents
    // ^^^^^^^^^^^^^^^^^^^^^^^^^^^^
    // VaultApp.app
    // ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
    // vault/bin/macos/
    // ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
    // vault/

    appPath := filepath.Dir(exePath) // .../Contents/MacOS
    contentsPath := filepath.Dir(appPath) // .../Contents
    bundlePath := filepath.Dir(contentsPath) // .../VaultApp.app
    binPath := filepath.Dir(bundlePath) // .../bin/macos
    vaultRoot := filepath.Dir(binPath) // vault/

    return vaultRoot
}
```

**Zusätzliche Besonderheiten macOS:**

| Thema | Herausforderung | Lösung |
|-------|-----------------|--------|
| Notarization | App nicht signiert → Gatekeeper blockiert | Developer-ID Signatur oder Developer Mode aktivieren |
| File Permission | Stick nicht als " lokaler Datenträger" erkannt | Hardened Runtime + com.apple.security.device.usb entitlement |
| Pfad mit Leerzeichen | `/Volumes/My USB/` | Quoted Path-Handling in allen Go-Funktionen |

---

### 3.4 Linux noexec-Mount-Flag

**Problem:**  
Viele Linux-Distributionen mounten USB-Sticks mit dem `noexec`-Flag, das die Ausführung von Binaries im Dateisystem verhindert:

```
/media/user/USB on /dev/sdb1 type vfat (rw,nosuid,nodev,noexec,...)
                                                        ^^^^^^^^
```

Die App startet nicht: `Permission denied` oder `Exec format error`.

**Lösung – Multi-Ansatz:**

```go
// CheckExecutablePermission prüft, ob die App ausführbar ist
func CheckExecutablePermission() error {
    exePath, err := os.Executable()
    if err != nil {
        return err
    }

    file, err := os.Open(exePath)
    if err != nil {
        return err
    }
    defer file.Close()

    // Versuche exec-Flag zu setzen
    if err := os.Chmod(exePath, 0755); err != nil {
        return fmt.Errorf("cannot make executable (noexec mount?): %w", err)
    }

    return nil
}

// Alternative: Wrapper-Skript, das vom Benutzer in PATH gelegt wird
// oder ein kleiner Shell-Starter, der in einem beschreibbaren tmp-Verzeichnis
// die eigentliche App kopiert und dort ausführt
```

**Empfohlene User-Lösung:**

```bash
# Skript zum Mounten mit exec-Flag (optional)
sudo mount -o remount,exec /media/user/USB

# Oder: Stick mit fstab-Eintrag für automatisches exec-Mount
# /etc/fstab:
# UUID=XXXX-XXXX /media/user/USB vfat defaults,exec 0 0
```

**Programmatische Fallback-Lösung für noexec:**

```go
// ExecuteFromTemp kopiert die App in ein ausführbares tmp-Verzeichnis
func ExecuteFromTemp() (string, error) {
    exePath, err := os.Executable()
    if err != nil {
        return "", err
    }

    tmpDir := os.TempDir()
    destPath := filepath.Join(tmpDir, "vaultapp-temp-"+uuid.New().String())

    src, err := os.Open(exePath)
    if err != nil {
        return "", err
    }
    defer src.Close()

    dst, err := os.Create(destPath)
    if err != nil {
        return "", err
    }
    defer dst.Close()

    if _, err := io.Copy(dst, src); err != nil {
        return "", err
    }

    os.Chmod(destPath, 0755)
    return destPath, nil
}
```

**Praktischer Hinweis:** Bei den meisten Linux-Desktops ist `noexec` nicht mehr standardmäßig gesetzt. Testen Sie vor der Implementierung.

---

## 4. Datenbank-Schema

### 4.1 SQLite-Tabellen

```sql
-- Grundtabellen für Datenträger und Dateien

CREATE TABLE drives (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid        TEXT UNIQUE NOT NULL,        -- Hardware-UUID des Datenträgers
    label       TEXT,                         -- Vom Benutzer vergebener Name
    serial      TEXT,                         -- Seriennummer
    vendor      TEXT,                         -- Hersteller
    model       TEXT,                         -- Modellbezeichnung
    total_size  INTEGER,                      -- Gesamtgröße in Bytes
    fs_type     TEXT,                         -- Dateisystem (exFAT, NTFS, HFS+)
    vault_path  TEXT,                         -- Pfad im Vault (z.B. "drives/usb-001")
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE files (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    drive_id    INTEGER NOT NULL REFERENCES drives(id),
    path        TEXT NOT NULL,                -- Relativer Pfad auf dem Datenträger
    filename    TEXT NOT NULL,
    extension   TEXT,
    size        INTEGER,                      -- Dateigröße in Bytes
    mime_type   TEXT,
    md5_hash    TEXT,                         -- Datei-Hash zur Duplikatsuche
    thumbnail   TEXT,                         -- Pfad zum Thumbnail (relativ)
    metadata    TEXT,                         -- EXIF, ID3 etc. als JSON
    created_at  DATETIME,
    modified_at DATETIME,
    scanned_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tags (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT UNIQUE NOT NULL,
    category    TEXT,                         -- z.B. "ai_generated", "manual", "system"
    color       TEXT,                         -- Hex-Farbcode für UI
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE file_tags (
    file_id     INTEGER NOT NULL REFERENCES files(id),
    tag_id      INTEGER NOT NULL REFERENCES tags(id),
    source      TEXT,                         -- "manual", "ai", "auto"
    confidence  REAL,                         -- AI-Konfidenz (0.0-1.0)
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (file_id, tag_id)
);

CREATE TABLE ai_summaries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    file_id     INTEGER UNIQUE REFERENCES files(id),
    summary     TEXT,                         -- AI-generierte Zusammenfassung
    model_name  TEXT,                         -- Verwendetes Modell
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_files_drive ON files(drive_id);
CREATE INDEX idx_files_extension ON files(extension);
CREATE INDEX idx_files_hash ON files(md5_hash);
CREATE INDEX idx_file_tags_file ON file_tags(file_id);
CREATE INDEX idx_file_tags_tag ON file_tags(tag_id);
```

### 4.2 Pfad-Handling in der Datenbank

```go
// Beim Speichern: Absoluter Pfad → Relativer Pfad
func SaveFilePath(drivePath, absoluteFilePath string) string {
    relPath, err := filepath.Rel(drivePath, absoluteFilePath)
    if err != nil {
        return absoluteFilePath // Fallback
    }
    return relPath
}

// Beim Auslesen: Relativer Pfad → Absoluter Pfad (nur für Anzeige)
func GetDisplayPath(drivePath, relativePath string) string {
    return filepath.Join(drivePath, relativePath)
}
```

---

## 5. AI-Integration (llama.cpp)

### 5.1 Architektur

```
┌──────────────────────────────────────────────────────┐
│  Wails Frontend (HTML/htmx)                         │
│  ┌────────────┐  ┌────────────┐  ┌──────────────┐   │
│  │   Scan     │  │   Search   │  │   Tagging    │   │
│  │   UI       │  │   UI       │  │   UI         │   │
│  └─────┬──────┘  └─────┬──────┘  └──────┬───────┘   │
└────────┼───────────────┼────────────────┼───────────┘
         │               │                │
         ▼               ▼                ▼
┌──────────────────────────────────────────────────────┐
│  Wails Backend (Go)                                 │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │
│  │ Scanner  │  │  Search  │  │   LLM Service     │  │
│  │ Service  │  │  Engine  │  │   (llama.cpp)     │  │
│  └────┬─────┘  └────┬─────┘  └─────────┬─────────┘  │
└───────┼─────────────┼──────────────────┼────────────┘
        │             │                  │
        ▼             ▼                  ▼
┌──────────────────────────────────────────────────────┐
│  Data Layer                                         │
│  ┌───────────┐  ┌──────────────────────────────────┐ │
│  │   SQLite  │  │   Local LLM (llama.cpp)          │ │
│  │   vault.db│  │   assets/models/*.gguf           │ │
│  └───────────┘  └──────────────────────────────────┘│
└──────────────────────────────────────────────────────┘
```

### 5.2 LLM-Service in Go (vereinfacht)

```go
// llm_service.go

type LLMService struct {
    modelPath string
    ctx       *llama.Context
    model     *llama.Model
}

// NewLLMService lädt ein GGUF-Modell
func NewLLMService(modelPath string) (*LLMService, error) {
    params := llama.DefaultParams()
    params.ContextSize = 2048 // Klein für embedded Nutzung

    model, err := llama.LoadModel(modelPath, params)
    if err != nil {
        return nil, err
    }

    return &LLMService{modelPath: modelPath, model: model}, nil
}

// GenerateTags erzeugt Tags für einen Dateinamen/Dateiinhalt
func (s *LLMService) GenerateTags(filename, mimeType, previewText string) ([]string, error) {
    prompt := fmt.Sprintf(
        "Analysiere diese Datei und schlage 5 passende Tags vor.\n"+
        "Dateiname: %s\n"+
        "Typ: %s\n"+
        "Vorschau: %s\n"+
        "Tags (kommagetrennt):",
        filename, mimeType, previewText,
    )

    // ... llama inference durchführen
    // ... Ergebnis parsen und Tags extrahieren
}

// GenerateSummary erstellt eine kurze Zusammenfassung
func (s *LLMService) GenerateSummary(fileContent string) (string, error) {
    // Analog zu GenerateTags
}
```

### 5.3 Empfohlene Modelle

| Modell | Parameter | Geeignet für | Größe (ca.) |
|--------|-----------|--------------|-------------|
| llama-3.2-1b | 1B | Dateinamen, einfache Tags | 700 MB |
| qwen2.5-1.5b | 1.5B | Bessere Texte, Metadaten | 1 GB |
| gemma-2-2b | 2B | Ausgewogenes Verhältnis | 1.5 GB |
| phi-3-mini | 3.8B | Höchste Qualität, langsamer | 2.5 GB |

---

## 6. Datenträger-Erkennung

### 6.1 Go-Bibliotheken für Hardware-IDs

**Windows:**
```go
// windows_drive.go
import "golang.org/x/sys/windows"

// GetDriveSerialNumber holt die Seriennummer über Win32 API
func GetDriveSerialNumber(driveLetter string) (string, error) {
    // WMI-Abfrage oder SetupAPI
}
```

**macOS:**
```go
// darwin_drive.go
import "github.com/karrick/partutil"

// Alternative: system_profiler SPUSBDataType
func GetDriveInfoBSD(devicePath string) (*DriveInfo, error) {
    // IOKit oder system_profiler CLI
}
```

**Linux:**
```go
// linux_drive.go

// GetDriveInfo liest /dev/disk/by-id/
func GetDriveInfoByID(devicePath string) (*DriveInfo, error) {
    // /dev/disk/by-id/ auslesen
    // lsblk --fs für UUID und Label
    // ls -l /dev/disk/by-uuid/
}
```

### 6.2 Automatische Überwachung

```go
// drive_watcher.go

// StartWatch überwacht Datenträger-Verbindungen (Hotplug)
func StartWatch(callback func(*DriveInfo, string)) error {
    switch runtime.GOOS {
    case "windows":
        return watchWindows(callback)
    case "darwin":
        return watchDarwin(callback)
    case "linux":
        return watchLinux(callback)
    }
}
```

---

## 7. Frontend (htmx + Go Templates)

### 7.1 Seitenstruktur

```
/ (index)
├── Scan: Neue Datenträger scannen
├── Library: Alle katalogisierten Dateien durchsuchen
├── Drives: Übersicht aller verwalteten Datenträger
└── Settings: Modell-Auswahl, Konfiguration
```

### 7.2 Go-Template-Struktur (vereinfacht)

```go
// page.go
type Drive struct {
    ID       int
    Label    string
    UUID     string
    FileCount int
}

type File struct {
    ID        int
    Filename  string
    Extension string
    Size      int64
    Thumbnail string
    Tags      []string
}
```

```html
<!-- templates/drives.html -->
<div class="drive-grid">
    {{range .Drives}}
    <div class="drive-card" hx-get="/drive/{{.ID}}" hx-target="#content">
        <h3>{{.Label}}</h3>
        <p>{{.FileCount}} Dateien</p>
        <span class="uuid">{{.UUID}}</span>
    </div>
    {{end}}
</div>

<button hx-post="/scan"
        hx-trigger="click"
        hx-swap="innerHTML"
        class="btn-primary">
    Neuen Datenträger scannen
</button>
```

---

## 8. Deployment-Szenarien

### 8.1 Stick-Variante (USB-Stick)

```
[USB-Stick]  (exFAT)
└── vault/
    ├── bin/
    │   ├── windows/VaultApp.exe
    │   ├── macos/VaultApp.app/
    │   └── linux/vaultapp
    ├── data/
    │   ├── vault.db
    │   └── config.json
    └── assets/
        ├── models/small-model.gguf
        └── thumbnails/

Anwendung: Stick einstecken → passende .exe/.app starten
```

### 8.2 Festplatten-Variante (empfohlen)

```
[Externe Festplatte]  (exFAT oder APFS)
└── VaultApp/
    ├── VaultApp.exe      # Windows: direkt in Hauptordner
    ├── VaultApp.app/     # macOS: direkt in Hauptordner
    ├── vault/            # Daten und Assets
    └── README.md

Anwendung: Festplatte einstecken → App starten
```

---

## 9. Konfigurationsdatei (config.json)

```json
{
    "version": "1.0",
    "defaults": {
        "model": "qwen2.5-1.5b.gguf",
        "thumbnailSize": 200,
        "autoScan": true
    },
    "paths": {
        "relative": true,
        "basePath": "auto"
    },
    "llm": {
        "provider": "local",
        "localModel": "assets/models/default-model.gguf",
        "contextSize": 2048,
        "fallbackProviders": ["ollama", "openrouter"]
    },
    "ui": {
        "theme": "dark",
        "language": "de",
        "thumbnailCache": "assets/thumbnails"
    }
}
```

---

## 10. Checkliste vor Build & Release

### Plattform-übergreifend
- [ ] Vault-Root-Erkennung für jede Plattform getestet
- [ ] Relative Pfade in allen Datenbank-Operationen
- [ ] SQLite mit exFAT-kompatiblen PRAGMAs
- [ ] Config-Erstellung beim ersten Start

### Windows
- [ ] Test auf exFAT-formatiertem Stick
- [ ] Hardware-UUID-Erkennung über Win32 API
- [ ] .exe im gleichen Ordner wie vault/ getestet

### macOS
- [ ] App-Bundle-Pfad drei Ebenen über MacOS/ ermittelt
- [ ] Test auf USB-Stick mit exFAT
- [ ] Notarization (optional für Developer-ID)

### Linux
- [ ] Test mit noexec-Mount (VM oder realer Stick)
- [ ] /dev/disk/by-uuid Erkennung funktioniert
- [ ] App-Icon im .desktop-File korrekt

---

## 11. Offene Fragen / ToDos

- [ ] Vision-Model Integration für Bild-Analyse (spätere Version)
- [ ] Verschlüsselung der SQLite-DB (optional)
- [ ] Multi-User-Support über Netzwerk (spätere Version)
- [ ] Sync-Mechanismus zwischen mehreren Vaults

---

*Dieses Dokument dient als technischer Leitfaden für die Implementierung der VaultApp.*