# TUI Wireframes and Visual Guide

This document provides visual representations of the modfetch TUI interface to help you understand the layout and navigation.

## Table of Contents

- [Overview](#overview)
- [Tab Layout](#tab-layout)
- [Download Tabs (0-4)](#download-tabs-0-4)
- [Library Tab (5)](#library-tab-5)
- [Settings Tab (6)](#settings-tab-6)
- [Navigation Flow](#navigation-flow)
- [Keyboard Shortcuts Summary](#keyboard-shortcuts-summary)

## Overview

The modfetch TUI has **7 tabs** organized into three functional groups:

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│  [0] All  [1] Pending  [2] Active  [3] Done  [4] Failed  [5] Library  [6] Settings  │
└─────────────────────────────────────────────────────────────────────────────────────┘
    └─────────────────────┬──────────────────────────────┘    └────┬────┘ └────┬────┘
                   Download Management                            Browse    Config
```

## Tab Layout

### Download Tabs (0-4): Download Management

```
╔══════════════════════════════════════════════════════════════════════════╗
║  modfetch v0.7.0                    Tab: [2] Active                      ║
╠══════════════════════════════════════════════════════════════════════════╣
║  Summary                                                                 ║
║  ┌──────────────────────────────────────────────────────────────────────┐║
║  │ 🔄 Active: 3   ✓ Completed: 15   ✗ Failed: 2   ⏳ Pending: 5        │║
║  │ Throughput: 45.2 MB/s   Total: 2.3 GB / 5.7 GB (40%)                 │║
║  │ Auth Status: HF ✓  Civ ✓                                             │║
║  └──────────────────────────────────────────────────────────────────────┘║
╠══════════════════════════════════════════════════════════════════════════╣
║  Downloads Table                                                         ║
║  ┌──────────────────────────────────────────────────────────────────────┐║
║  │ Status      │ Progress    │ Speed      │ ETA    │ Size  │ File       │║
║  ├──────────────────────────────────────────────────────────────────────┤║
║  │▶ Running    │ ████████░░░ │ 15.2 MB/s  │ 2m 15s │ 3.8GB │ llama-2... │║
║  │  Running    │ ██████░░░░░ │ 12.8 MB/s  │ 3m 42s │ 4.1GB │ mistral... │║
║  │  Running    │ ███░░░░░░░░ │ 17.4 MB/s  │ 5m 08s │ 6.9GB │ sdxl...    │║
║  │  Planning   │ ...         │ -          │ -      │ 2.2GB │ flux...    │║
║  └──────────────────────────────────────────────────────────────────────┘║
╠══════════════════════════════════════════════════════════════════════════╣
║  ↑↓:Select  n:New  y:Retry  p:Pause  D:Delete  O:Open  /:Filter  ?:Help  ║
╚══════════════════════════════════════════════════════════════════════════╝
```

### Library Tab (5): Model Browser

```
╔═══════════════════════════════════════════════════════════════════════════╗
║  modfetch v0.7.0                    Tab: [5] Library                      ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Model Library                                                            ║
║  ┌──────────────────────────────────────────────────────────────────────┐ ║
║  │ Search: "llama"         Filter: Type=LLM, Source=huggingface         │ ║
║  │ Showing: 3 of 127 models                                             │ ║
║  └──────────────────────────────────────────────────────────────────────┘ ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Models List                                                              ║
║  ┌──────────────────────────────────────────────────────────────────────┐ ║
║  │                                                                      │ ║
║  │  ★ llama-2-7b-chat                                                   │ ║
║  │     LLM • 3.8 GB • Q4_K_M • huggingface                              │ ║
║  │                                                                      │ ║
║  │▶   llama-2-13b-instruct                                              │ ║
║  │     LLM • 7.4 GB • Q5_K_S • huggingface                              │ ║
║  │                                                                      │ ║
║  │    llama-3-8b-base                                                   │ ║
║  │     LLM • 4.2 GB • Q4_K_M • huggingface                              │ ║
║  │                                                                      │ ║
║  └──────────────────────────────────────────────────────────────────────┘ ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  ↑↓:Select  Enter:Details  /:Search  f:Favorite  S:Scan  L:List  ?:Help   ║
╚═══════════════════════════════════════════════════════════════════════════╝
```

### Library Detail View

```
╔═══════════════════════════════════════════════════════════════════════════╗
║  modfetch v0.7.0                    Model Details                         ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  llama-2-13b-instruct                                            ★ Favorite║
║  ┌──────────────────────────────────────────────────────────────────────┐║
║  │ Basic Info                                                             │║
║  │ Type: LLM                    Source: huggingface                       │║
║  │ Version: v2.0                Author: Meta                              │║
║  │ License: MIT                                                           │║
║  │                                                                        │║
║  │ Specifications                                                         │║
║  │ Architecture: Llama 2        Parameters: 13B                          │║
║  │ Quantization: Q5_K_S         Base Model: Llama-2                      │║
║  │                                                                        │║
║  │ File Information                                                       │║
║  │ Size: 7.4 GB                 Format: .gguf                            │║
║  │ Path: /home/user/models/llm/llama-2-13b-instruct.Q5_K_S.gguf         │║
║  │                                                                        │║
║  │ Description                                                            │║
║  │ A 13 billion parameter language model fine-tuned for instruction      │║
║  │ following. Optimized for chat and task completion with improved       │║
║  │ accuracy over the 7B variant.                                         │║
║  │                                                                        │║
║  │ Tags                                                                   │║
║  │ llm, text-generation, instruction-following, chat, meta               │║
║  │                                                                        │║
║  │ Links                                                                  │║
║  │ Homepage: https://huggingface.co/meta/llama-2-13b-instruct            │║
║  └──────────────────────────────────────────────────────────────────────┘║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Esc:Back  f:Toggle Favorite  Q:Quit                                     ║
╚═══════════════════════════════════════════════════════════════════════════╝
```

### Settings Tab (6): Configuration View

```
╔═══════════════════════════════════════════════════════════════════════════╗
║  modfetch v0.7.0                    Tab: [6] Settings                     ║
╠═══════════════════════════════════════════════════════════════════════════╣
║  Configuration                                                            ║
║  ┌──────────────────────────────────────────────────────────────────────┐║
║  │ Directories                                                            │║
║  │ ┌────────────────────────────────────────────────────────────────────┤║
║  │ │ Data Root:     /home/user/modfetch-data                            │║
║  │ │ Download Root: /home/user/Downloads/modfetch                       │║
║  │ │ Placement:     symlink                                             │║
║  │ └────────────────────────────────────────────────────────────────────┤║
║  │                                                                        │║
║  │ API Sources                                                            │║
║  │ ┌────────────────────────────────────────────────────────────────────┤║
║  │ │ HuggingFace:  ✓ Enabled   HF_TOKEN: ✓ Set   Auth: ✓ Valid        │║
║  │ │ CivitAI:      ✓ Enabled   CIVITAI_TOKEN: ✓ Set   Auth: ✓ Valid   │║
║  │ └────────────────────────────────────────────────────────────────────┤║
║  │                                                                        │║
║  │ Download Settings                                                      │║
║  │ ┌────────────────────────────────────────────────────────────────────┤║
║  │ │ Chunks per file: 4                                                 │║
║  │ │ Chunk size: 8 MB                                                   │║
║  │ │ Timeout: 60s                                                       │║
║  │ └────────────────────────────────────────────────────────────────────┤║
║  │                                                                        │║
║  │ Placement Rules                                                        │║
║  │ ┌────────────────────────────────────────────────────────────────────┤║
║  │ │ • *.gguf, *.ggml → /opt/ollama/models                              │║
║  │ │ • *.safetensors → /opt/comfyui/models/checkpoints                  │║
║  │ │ • *lora* → /opt/comfyui/models/loras                               │║
║  │ └────────────────────────────────────────────────────────────────────┤║
║  └──────────────────────────────────────────────────────────────────────┘║
╠═══════════════════════════════════════════════════════════════════════════╣
║  ↑↓:Scroll  Esc:Back  Q:Quit                  Edit config file to change ║
╚═══════════════════════════════════════════════════════════════════════════╝
```

## Navigation Flow

```
                              modfetch TUI
                                   │
                    ┌──────────────┼──────────────┐
                    │              │              │
              Download Tabs    Library Tab   Settings Tab
                    │              │              │
        ┌───────────┼────────┐     │              │
        │           │        │     │              │
    All Tabs   Pending   Active   │              │
        │       Tab 1    Tab 2    │              │
    Completed  Failed             │              │
     Tab 3     Tab 4              │              │
                                  │              │
                              ┌───┴───┐          │
                              │       │          │
                          List View  Detail      │
                          (Press 5)  (Enter)     │
                              │       │          │
                              └───────┘          │
                                                 │
                                         (Press 6 or M)
```

### Navigation Between Tabs

```
┌─────────────────────────────────────────────────────────────────┐
│                      TAB NAVIGATION                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Press 0-6 (Number Keys):                                       │
│  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐  ┌──────┐               │
│  │  0   │→ │  1   │→ │  2   │→ │  3   │→ │  4   │               │
│  │ All  │  │Pend  │  │Active│  │Done  │  │Failed│               │
│  └──────┘  └──────┘  └──────┘  └──────┘  └──────┘               │
│                                                                 │
│  Quick Jump Keys:                                               │
│  ┌──────┐            ┌──────┐                                   │
│  │  5   │            │  6   │                                   │
│  │  L   │ Library    │  M   │ Settings                          │
│  └──────┘            └──────┘                                   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Common Actions Flow

```
┌──────────────────────────────────────────────────────────────────┐
│                     DOWNLOAD WORKFLOW                            │
└──────────────────────────────────────────────────────────────────┘

   Start                                              Complete
     │                                                    │
     ├──► Press 'n' (New Download)                        │
     │         │                                          │
     │         ├──► Enter URL                             │
     │         │                                          │
     │         ├──► Confirm Destination                   │
     │         │                                          │
     │         └──► Download Starts                       │
     │                │                                   │
     │                ├──► Tab 1: Pending                 │
     │                │         │                         │
     │                ├──► Tab 2: Active (Downloading)    │
     │                │         │                         │
     │                │    ┌────┴────┐                    │
     │                │    │ Actions │                    │
     │                │    │  p=Pause│                    │
     │                │    │  y=Resume                    │
     │                │    └─────────┘                    │
     │                │         │                         │
     │                ├──► Tab 3: Completed ──────────────┤
     │                │    OR                             │
     │                └──► Tab 4: Failed                  │
     │                          │                         │
     │                          └──► y (Retry)            │
     │                                                    │
     └────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────┐
│                     LIBRARY WORKFLOW                             │
└──────────────────────────────────────────────────────────────────┘

   Browse Models                                   Manage
     │                                                │
     ├──► Press '5' or 'L' (Open Library)             │
     │         │                                      │
     │         ├──► View Model List                   │
     │         │         │                            │
     │         │    ┌────┴────┐                       │
     │         │    │ Actions │                       │
     │         │    │ /:Search│                       │
     │         │    │ S:Scan  │                       │
     │         │    │ f:Fav   │                       │
     │         │    └─────────┘                       │
     │         │         │                            │
     │         ├──► Press Enter (View Details)        │
     │         │         │                            │
     │         │         ├──► Read Description        │
     │         │         ├──► Check Specs             │
     │         │         ├──► f: Toggle Favorite ─────┤
     │         │         │                            │
     │         │         └──► Esc (Back to List)      │
     │         │                                      │
     │         └──► Press 'S' (Scan Directories) ─────┤
     │                                                │
     └────────────────────────────────────────────────┘
```

## Keyboard Shortcuts Summary

### Visual Cheatsheet

```
╔═══════════════════════════════════════════════════════════════════════╗
║               MODFETCH TUI KEYBOARD SHORTCUTS                         ║
╠═══════════════════════════════════════════════════════════════════════╣
║  GLOBAL                                                               ║
║  ┌─────────────────────────────────────────────────────────────────┐ ║
║  │ 0-4      Switch to download tabs (All/Pending/Active/Done/Fail) │ ║
║  │ 5 or L   Library tab                                            │ ║
║  │ 6 or M   Settings tab                                           │ ║
║  │ ?        Help overlay                                           │ ║
║  │ q or Q   Quit                                                   │ ║
║  │ j/k      Navigate down/up (Vim-style)                           │ ║
║  │ ↑/↓      Navigate up/down (Arrow keys)                          │ ║
║  └─────────────────────────────────────────────────────────────────┘ ║
╠═══════════════════════════════════════════════════════════════════════╣
║  DOWNLOAD TABS (0-4)                                                  ║
║  ┌─────────────────────────────────────────────────────────────────┐ ║
║  │ n        New download                                           │ ║
║  │ b        Import batch file                                      │ ║
║  │ y or r   Start/retry download                                   │ ║
║  │ p        Pause/cancel download                                  │ ║
║  │ D        Delete download                                        │ ║
║  │ O        Open/reveal file in file manager                       │ ║
║  │ C        Copy path to clipboard                                 │ ║
║  │ U        Copy URL to clipboard                                  │ ║
║  │ /        Filter by substring                                    │ ║
║  │ s        Sort by speed                                          │ ║
║  │ e        Sort by ETA                                            │ ║
║  │ R        Sort by remaining bytes                                │ ║
║  │ o        Clear sort                                             │ ║
║  │ g        Group by status                                        │ ║
║  │ t        Toggle column view (URL/DEST/HOST)                     │ ║
║  │ v        Toggle compact view                                    │ ║
║  │ i        Toggle inspector                                       │ ║
║  │ T        Cycle theme (default/neon/dracula/solarized)           │ ║
║  │ X        Clear ephemeral row                                    │ ║
║  └─────────────────────────────────────────────────────────────────┘ ║
╠═══════════════════════════════════════════════════════════════════════╣
║  LIBRARY TAB (5)                                                      ║
║  ┌─────────────────────────────────────────────────────────────────┐ ║
║  │ Enter    View model details                                     │ ║
║  │ Esc      Back to list (from detail view)                        │ ║
║  │ /        Search models by name                                  │ ║
║  │ f        Toggle favorite                                        │ ║
║  │ S        Scan directories for models                            │ ║
║  │ F        Toggle filter menu                                     │ ║
║  └─────────────────────────────────────────────────────────────────┘ ║
╠═══════════════════════════════════════════════════════════════════════╣
║  SETTINGS TAB (6)                                                     ║
║  ┌─────────────────────────────────────────────────────────────────┐ ║
║  │ j/k ↑↓   Scroll settings                                        │ ║
║  │ Esc      Back to downloads                                      │ ║
║  └─────────────────────────────────────────────────────────────────┘ ║
╚═══════════════════════════════════════════════════════════════════════╝
```

### Key Categories

```
┌─────────────────────────────────────────────────────────┐
│  NAVIGATION           DOWNLOADS        LIBRARY          │
│  ───────────          ─────────        ───────          │
│  0-6  Tabs            n  New           5/L  Open        │
│  j/k  Up/Down         y  Start         /    Search      │
│  ?    Help            p  Pause         f    Favorite    │
│  q    Quit            D  Delete        S    Scan        │
│                       O  Open          Enter Details    │
│                       C  Copy Path                      │
│                       U  Copy URL                       │
│                       /  Filter                         │
│                       s  Sort Speed                     │
│                       e  Sort ETA                       │
└─────────────────────────────────────────────────────────┘
```

## Status Icons Reference

```
┌───────────────────────────────────────────────────────────┐
│  DOWNLOAD STATUS                                          │
│  ───────────────                                          │
│  ⏳ Pending         Waiting to start                      │
│  🔄 Running         Download in progress                  │
│  ⚙️  Planning        Calculating chunks                   │
│  ✓  Completed       Successfully downloaded               │
│  ✗  Failed          Download error                        │
│  ⏸  Paused          Manually paused                       │
│  🔒 Hold(auth)      Authentication required               │
│  🚫 Hold(rl)        Rate limited                          │
└───────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│  LIBRARY INDICATORS                                       │
│  ──────────────────                                       │
│  ★  Favorite        Marked as favorite                    │
│  ▶  Selected        Currently selected item               │
│  🟢 HuggingFace     Source: HuggingFace Hub               │
│  🟣 CivitAI         Source: CivitAI                       │
│  ⚪ Local           Source: Local scan                    │
└───────────────────────────────────────────────────────────┘

┌───────────────────────────────────────────────────────────┐
│  AUTH STATUS                                              │
│  ───────────                                              │
│  ✓  Valid           Token authenticated                   │
│  ✗  Invalid         Token rejected/missing                │
│  -  Not Set         Token environment variable not set    │
└───────────────────────────────────────────────────────────┘
```

## Tips for Navigation

### Quick Start Flow

```
1. Launch TUI
   └─► modfetch tui --config config.yml

2. First time? Check settings
   └─► Press '6' or 'M'
       └─► Verify paths and token status

3. Browse your models
   └─► Press '5' or 'L'
       └─► See what you have downloaded
       └─► Press 'S' to scan directories

4. Download a model
   └─► Press '0' (All tab)
       └─► Press 'n' (New download)
       └─► Paste URL
       └─► Watch in Active tab (Press '2')

5. Organize your library
   └─► Press '5' or 'L'
       └─► Mark favorites with 'f'
       └─► Search with '/'
```

### Pro Tips

```
┌──────────────────────────────────────────────────────────────┐
│  💡 PRO TIPS                                                 │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  • Use '/' to filter downloads by name or URL                │
│  • Press 's' to see which downloads are fastest              │
│  • Press 'e' to see which will finish soonest                │
│  • Use Tab 2 (Active) to monitor ongoing downloads           │
│  • Press 'O' to open completed files in file manager         │
│  • Use 'C' to copy paths for use in other apps               │
│  • Press 'S' in Library to discover existing models          │
│  • Mark favorites with 'f' for quick access                  │
│  • Press '?' anytime to see context help                     │
│  • Use 'T' to cycle through visual themes                    │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## See Also

- [TUI Guide](TUI_GUIDE.md) - Complete TUI documentation
- [Library Guide](LIBRARY.md) - Library feature details
- [User Guide](USER_GUIDE.md) - General usage guide
- [Configuration](CONFIG.md) - Config file reference
