# Quick Start Guide

Get up and running with modfetch in **5 minutes**! This visual guide will walk you through installation, configuration, and your first download.

## Table of Contents

- [Step 1: Install](#step-1-install)
- [Step 2: Configure](#step-2-configure)
- [Step 3: Your First Download](#step-3-your-first-download)
- [Step 4: Explore the TUI](#step-4-explore-the-tui)
- [Step 5: Browse Your Library](#step-5-browse-your-library)
- [Next Steps](#next-steps)

---

## Step 1: Install

### Option A: One-line Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/jxwalker/modfetch/main/scripts/install.sh | bash
```

```
✓ Downloaded modfetch v0.6.0
✓ Installed to /usr/local/bin/modfetch
✓ Ready to use!
```

### Option B: Build from Source

```bash
git clone https://github.com/jxwalker/modfetch
cd modfetch
make build
```

**Result:**
```
Binary created at: ./bin/modfetch
```

Add to PATH (optional):
```bash
export PATH="$PWD/bin:$PATH"
```

---

## Step 2: Configure

### Quick Config

Create a minimal configuration file:

```bash
mkdir -p ~/.config/modfetch
cat > ~/.config/modfetch/config.yml << 'YAML'
version: 1
general:
  data_root: "~/modfetch-data"
  download_root: "~/Downloads/modfetch"
  placement_mode: "symlink"
network:
  timeout_seconds: 60
concurrency:
  per_file_chunks: 4
  chunk_size_mb: 8
sources:
  huggingface: { enabled: true, token_env: "HF_TOKEN" }
  civitai:     { enabled: true, token_env: "CIVITAI_TOKEN" }
YAML
```

**Visual Structure:**
```
~/.config/modfetch/
└── config.yml          ← Your configuration

~/Downloads/modfetch/   ← Downloaded models go here
~/modfetch-data/        ← Database and state
```

### Set Default Config

```bash
export MODFETCH_CONFIG=~/.config/modfetch/config.yml
```

Add to your `~/.bashrc` or `~/.zshrc` to make it permanent!

### (Optional) API Tokens

Only needed for private/gated content:

```bash
export HF_TOKEN="your_huggingface_token"       # For HuggingFace
export CIVITAI_TOKEN="your_civitai_token"      # For CivitAI
```

**Where to get tokens:**
- **HuggingFace:** https://huggingface.co/settings/tokens
- **CivitAI:** https://civitai.com/user/account

---

## Step 3: Your First Download

Let's download a test file to verify everything works!

### Command Line Download

```bash
modfetch download --url 'https://proof.ovh.net/files/1Mb.dat'
```

**What you'll see:**
```
┌─────────────────────────────────────────────────────────┐
│ Downloading: 1Mb.dat                                    │
│ Progress: ████████████████████ 100%                     │
│ Speed: 15.2 MB/s                                        │
│ ETA: Complete!                                          │
│                                                         │
│ ✓ Download complete                                     │
│ ✓ SHA256 verified                                       │
│ ✓ Size: 1.00 MB                                         │
│ ✓ Time: 0.07s                                           │
│ ✓ Saved: ~/Downloads/modfetch/1Mb.dat                   │
└─────────────────────────────────────────────────────────┘
```

### Download from HuggingFace

```bash
modfetch download --url 'hf://TheBloke/Llama-2-7B-GGUF/llama-2-7b.Q4_K_M.gguf'
```

### Download from CivitAI

```bash
modfetch download --url 'civitai://model/123456'
```

**Resolver Magic:**
```
Input:  civitai://model/123456
        ↓
Resolve: Find latest version
        ↓
        Find primary file
        ↓
        Get download URL + metadata
        ↓
Output: ~/Downloads/modfetch/ModelName - filename.safetensors
```

---

## Step 4: Explore the TUI

The **Terminal User Interface (TUI)** gives you a real-time dashboard for managing downloads.

### Launch the TUI

```bash
modfetch tui
```

### Your First Look

```
╔═══════════════════════════════════════════════════════════════════════════╗
║  modfetch v0.6.0                    Tab: [0] All                          ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Summary                                                                  ║
║  ┌──────────────────────────────────────────────────────────────────────┐║
║  │ ✓ Completed: 1   🔄 Active: 0   ⏳ Pending: 0   ✗ Failed: 0         │║
║  │ Auth Status: HF ✓  Civ ✓                                             │║
║  └──────────────────────────────────────────────────────────────────────┘║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Downloads Table                                                          ║
║  ┌──────────────────────────────────────────────────────────────────────┐║
║  │ Status      │ Progress  │ Speed  │ ETA  │ Size   │ File              │║
║  ├──────────────────────────────────────────────────────────────────────┤║
║  │▶ Completed  │ 100%      │ -      │ -    │ 1.0 MB │ 1Mb.dat           │║
║  └──────────────────────────────────────────────────────────────────────┘║
╠═══════════════════════════════════════════════════════════════════════════╣
║  n:New  y:Retry  p:Pause  D:Delete  ?:Help  q:Quit                       ║
╚═══════════════════════════════════════════════════════════════════════════╝
```

### Navigate the Tabs

**Press number keys to switch tabs:**

```
┌──────────────────────────────────────────────────┐
│  0: All       ← All downloads                    │
│  1: Pending   ← Waiting to start                 │
│  2: Active    ← Currently downloading            │
│  3: Done      ← Successfully completed           │
│  4: Failed    ← Errors (retry with 'y')          │
│  5: Library   ← Browse your models (Press L)     │
│  6: Settings  ← View configuration (Press M)     │
└──────────────────────────────────────────────────┘
```

### Start a New Download from TUI

**Step-by-step:**

1. **Press `n`** (New download)
   ```
   ┌────────────────────────────────────────┐
   │ New Download                           │
   ├────────────────────────────────────────┤
   │ URL: _                                 │
   │                                        │
   │ Enter/Tab: Continue  Esc: Cancel       │
   └────────────────────────────────────────┘
   ```

2. **Paste your URL**
   ```
   ┌────────────────────────────────────────┐
   │ New Download                           │
   ├────────────────────────────────────────┤
   │ URL: hf://TheBloke/Llama-2-7B-GGUF/.. │
   │                                        │
   │ Enter/Tab: Continue  Esc: Cancel       │
   └────────────────────────────────────────┘
   ```

3. **Confirm destination** (or edit it)
   ```
   ┌────────────────────────────────────────┐
   │ New Download                           │
   ├────────────────────────────────────────┤
   │ URL: hf://TheBloke/Llama-2-7B-GGUF/.. │
   │ Dest: ~/Downloads/modfetch/llama-2-7b..│
   │                                        │
   │ Enter: Start  Esc: Cancel              │
   └────────────────────────────────────────┘
   ```

4. **Press Enter** - Download starts!

5. **Watch it in Tab 2** (Active)
   ```
   Press '2' to see:

   ║ Status    │ Progress    │ Speed     │ ETA    │ Size  │ File       ║
   ║ Running   │ ████░░░░░░░ │ 15.2 MB/s │ 3m 15s │ 3.8GB │ llama-2... ║
   ```

### Useful Actions

| Key | Action                | Tab         |
|-----|-----------------------|-------------|
| `n` | New download          | Any         |
| `y` | Retry failed download | Failed (4)  |
| `p` | Pause download        | Active (2)  |
| `O` | Open file             | Done (3)    |
| `C` | Copy path             | Any         |
| `/` | Filter/search         | Any         |
| `s` | Sort by speed         | Active (2)  |
| `e` | Sort by ETA           | Active (2)  |
| `?` | Help                  | Any         |
| `q` | Quit                  | Any         |

---

## Step 5: Browse Your Library

The **Library** tab lets you browse, search, and organize all your downloaded models.

### Open Library

**Press `5` or `L`** from any tab

```
╔═══════════════════════════════════════════════════════════════════════════╗
║  modfetch v0.6.0                    Tab: [5] Library                      ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Model Library                      Showing: 1 of 1 models                ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  ┌──────────────────────────────────────────────────────────────────────┐║
║  │                                                                        │║
║  │▶ llama-2-7b.Q4_K_M.gguf                                               │║
║  │   LLM • 3.8 GB • Q4_K_M • huggingface                                 │║
║  │                                                                        │║
║  └──────────────────────────────────────────────────────────────────────┘║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Enter:Details  /:Search  f:Favorite  S:Scan  ?:Help                     ║
╚═══════════════════════════════════════════════════════════════════════════╝
```

### Scan for Models

If you have existing models in your directories, discover them:

**Press `S`** in Library tab

```
Scanning directories...
 • ~/Downloads/modfetch/
 • ~/.ollama/models/
 • /opt/comfyui/models/

Found: 42 models (15 new, 27 existing)
✓ Scan complete
```

### View Model Details

**Press `Enter`** on any model

```
╔═══════════════════════════════════════════════════════════════════════════╗
║  llama-2-7b.Q4_K_M.gguf                                          ★ Favorite║
║  ┌──────────────────────────────────────────────────────────────────────┐║
║  │ Basic Info                                                             │║
║  │ Type: LLM                    Source: huggingface                       │║
║  │ Version: main                Author: TheBloke                          │║
║  │                                                                        │║
║  │ Specifications                                                         │║
║  │ Architecture: Llama 2        Parameters: 7B                           │║
║  │ Quantization: Q4_K_M         Base Model: Llama-2                      │║
║  │                                                                        │║
║  │ File Information                                                       │║
║  │ Size: 3.8 GB                 Format: .gguf                            │║
║  │ Path: ~/Downloads/modfetch/llama-2-7b.Q4_K_M.gguf                     │║
║  │                                                                        │║
║  │ Description                                                            │║
║  │ Llama 2 is a family of LLMs fine-tuned for dialogue. The 7B model     │║
║  │ uses Q4_K_M quantization for efficient inference while maintaining    │║
║  │ good quality.                                                          │║
║  │                                                                        │║
║  │ Links                                                                  │║
║  │ Homepage: https://huggingface.co/TheBloke/Llama-2-7B-GGUF             │║
║  └──────────────────────────────────────────────────────────────────────┘║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Esc:Back  f:Toggle Favorite  Q:Quit                                     ║
╚═══════════════════════════════════════════════════════════════════════════╝
```

### Search Your Library

**Press `/`** to search

```
┌────────────────────────────────────┐
│ Search: llama_                     │
└────────────────────────────────────┘

Results: 3 models
  ▶ llama-2-7b.Q4_K_M.gguf
    llama-2-13b-chat.Q5_K_S.gguf
    llama-3-8b-instruct.Q4_K_M.gguf
```

### Mark Favorites

**Press `f`** on any model to mark/unmark as favorite

```
★ = Favorite
  = Normal
```

---

## Next Steps

Congratulations! You now know the basics of modfetch. Here's what to explore next:

### 1. Check Settings

**Press `6` or `M`** to view your configuration:
- Directory paths
- API token status
- Placement rules
- Download settings

### 2. Set Up Placement

Automatically organize models by type:

```yaml
# Add to config.yml
placement:
  apps:
    ollama:
      base: /opt/ollama/models
      patterns:
        - "*.gguf"
        - "*.ggml"

    comfyui:
      base: /opt/comfyui
      paths:
        - models/checkpoints
        - models/loras
        - models/vae
      patterns:
        - "*.safetensors"
        - "*.ckpt"
```

**Then use:**
```bash
modfetch place --path ~/Downloads/modfetch/model.gguf
```

### 3. Batch Downloads

Create a batch file for multiple downloads:

```yaml
# jobs.yml
items:
  - url: hf://TheBloke/Llama-2-7B-GGUF/llama-2-7b.Q4_K_M.gguf
    dest: llama-2-7b.gguf

  - url: civitai://model/123456
    dest: sdxl-model.safetensors

  - url: https://example.com/model.bin
    sha256: abc123...
```

**Run batch:**
```bash
modfetch download --batch jobs.yml --place
```

### 4. Advanced Features

```bash
# Verify checksums
modfetch verify --all

# Deep-verify safetensors
modfetch verify --scan-dir ~/models --safetensors-deep

# Clean old partial downloads
modfetch clean --days 7

# Dry-run (preview without downloading)
modfetch download --url 'hf://...' --dry-run

# JSON output for scripting
modfetch download --url 'https://...' --summary-json
```

---

## Keyboard Shortcuts Cheatsheet

```
╔═══════════════════════════════════════════════════════════╗
║  ESSENTIAL SHORTCUTS                                      ║
╠═══════════════════════════════════════════════════════════╣
║  NAVIGATION               ACTIONS                         ║
║  0-4  Download tabs       n  New download                 ║
║  5/L  Library            y  Start/retry                   ║
║  6/M  Settings           p  Pause                         ║
║  j/k  Up/Down            D  Delete                        ║
║  ?    Help               O  Open file                     ║
║  q    Quit               /  Search/filter                 ║
╚═══════════════════════════════════════════════════════════╝
```

---

## Troubleshooting

### Common Issues

**Problem:** `modfetch: command not found`
```bash
# Solution: Add to PATH
export PATH="/usr/local/bin:$PATH"
# Or use full path
/usr/local/bin/modfetch --version
```

**Problem:** Authentication errors (401/403)
```bash
# Solution: Set API tokens
export HF_TOKEN="your_token"
export CIVITAI_TOKEN="your_token"

# Verify in Settings tab (Press 6 or M)
```

**Problem:** Library shows no models
```bash
# Solution: Scan directories
# In TUI: Press 5 (Library) then S (Scan)
# Or check your config paths match where files are
```

**Problem:** Slow downloads
```bash
# Solution: Adjust concurrency in config.yml
concurrency:
  per_file_chunks: 8    # Increase chunks
  chunk_size_mb: 16     # Increase chunk size
```

---

## Getting Help

- **Full Documentation:** See [docs/](.) folder
  - [TUI Guide](TUI_GUIDE.md) - Detailed TUI documentation
  - [TUI Wireframes](TUI_WIREFRAMES.md) - Visual interface guide
  - [Library Guide](LIBRARY.md) - Library features
  - [User Guide](USER_GUIDE.md) - Complete usage guide
  - [Config Reference](CONFIG.md) - Configuration options

- **Interactive Help:** Press `?` in the TUI anytime

- **Issues:** https://github.com/jxwalker/modfetch/issues

---

## Quick Reference Card

Save this for easy reference:

```
┌──────────────────────────────────────────────────────────┐
│  MODFETCH QUICK REFERENCE                                │
├──────────────────────────────────────────────────────────┤
│  CLI                                                     │
│  modfetch download --url 'URL'      Download a file     │
│  modfetch tui                       Launch TUI          │
│  modfetch verify --all              Verify downloads    │
│  modfetch place --path FILE         Place into app      │
│                                                          │
│  TUI                                                     │
│  0-4    Download tabs               L/5  Library        │
│  M/6    Settings                    n    New download   │
│  j/k    Navigate                    y    Retry          │
│  /      Search                      f    Favorite       │
│  Enter  Details                     S    Scan dirs      │
│  ?      Help                        q    Quit           │
│                                                          │
│  URLs                                                    │
│  hf://org/repo/file?rev=main        HuggingFace         │
│  civitai://model/ID                 CivitAI             │
│  https://example.com/file           Direct URL          │
└──────────────────────────────────────────────────────────┘
```

---

**Ready to download? Start with:** `modfetch tui` 🚀
