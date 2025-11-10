package tui

import (
	"bufio"
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jxwalker/modfetch/internal/downloader"
	"github.com/jxwalker/modfetch/internal/logging"
	"github.com/jxwalker/modfetch/internal/metadata"
	"github.com/jxwalker/modfetch/internal/placer"
	"github.com/jxwalker/modfetch/internal/resolver"
	"github.com/jxwalker/modfetch/internal/state"
)

// Download actions and commands

func (m *Model) recoverCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.st.ListDownloads()
		if err != nil {
			return recoverRowsMsg{rows: nil}
		}
		var todo []state.DownloadRow
		for _, r := range rows {
			st := strings.ToLower(strings.TrimSpace(r.Status))
			if st == "running" || st == "hold" {
				// Skip completed
				todo = append(todo, r)
			}
		}
		return recoverRowsMsg{rows: todo}
	}
}

func (m *Model) selectionOrCurrent() []state.DownloadRow {
	rows := m.visibleRows()
	if len(m.selectedKeys) == 0 {
		if m.selected >= 0 && m.selected < len(rows) {
			return []state.DownloadRow{rows[m.selected]}
		}
		return nil
	}
	var out []state.DownloadRow
	for _, r := range rows {
		if m.selectedKeys[keyFor(r)] {
			out = append(out, r)
		}
	}
	return out
}

func (m *Model) importBatchFile(path string) tea.Cmd {
	f, err := os.Open(path)
	if err != nil {
		m.addToast("batch open failed: " + err.Error())
		return nil
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	count := 0
	cmds := make([]tea.Cmd, 0, 64)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		u := parts[0]
		dest := ""
		typeOv := ""
		place := false
		for _, tok := range parts[1:] {
			if i := strings.IndexByte(tok, '='); i > 0 {
				k := strings.ToLower(strings.TrimSpace(tok[:i]))
				v := strings.TrimSpace(tok[i+1:])
				switch k {
				case "dest":
					dest = v
				case "type":
					typeOv = v
				case "place":
					v2 := strings.ToLower(v)
					place = v2 == "y" || v2 == "yes" || v2 == "true" || v2 == "1"
				}
			}
		}
		if dest == "" {
			dest = m.computeDefaultDest(u)
		}
		// Set placement data before spawning goroutine
		key := u + "|" + dest
		if strings.TrimSpace(typeOv) != "" {
			m.placeType[key] = typeOv
		}
		if place {
			m.autoPlace[key] = true
		}
		// Capture placement data to pass (avoid concurrent map access)
		autoPlace := m.autoPlace[key]
		placeType := m.placeType[key]
		cmds = append(cmds, m.startDownloadCmd(u, dest, autoPlace, placeType))
		count++
	}
	if err := sc.Err(); err != nil {
		m.addToast("batch read error: " + err.Error())
	}
	m.addToast(fmt.Sprintf("batch: started %d", count))
	return tea.Batch(cmds...)
}

func (m *Model) startDownloadCmd(urlStr, dest string, autoPlace bool, placeType string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.running[urlStr+"|"+dest] = cancel
	return m.downloadOrHoldCmd(ctx, urlStr, dest, true, autoPlace, placeType)
}

// probeSelectedCmd: probe reachability for rows without starting

func (m *Model) probeSelectedCmd(rows []state.DownloadRow) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(rows))
	for _, r := range rows {
		row := r
		cmds = append(cmds, func() tea.Msg {
			ctx := context.Background()
			headers := map[string]string{}
			if u, err := neturl.Parse(row.URL); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "huggingface.co") && m.cfg.Sources.HuggingFace.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
					if env == "" {
						env = "HF_TOKEN"
					}
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					}
				}
				if hostIs(h, "civitai.com") && m.cfg.Sources.CivitAI.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
					if env == "" {
						env = "CIVITAI_TOKEN"
					}
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					}
				}
			}
			reach, info := downloader.CheckReachable(ctx, m.cfg, row.URL, headers)
			if !reach {
				_ = m.st.UpsertDownload(state.DownloadRow{URL: row.URL, Dest: row.Dest, Status: "hold", LastError: info})
			}
			return probeMsg{url: row.URL, dest: row.Dest, reachable: reach, info: info}
		})
	}
	return tea.Batch(cmds...)
}

func (m *Model) startDownloadCmdCtx(ctx context.Context, urlStr, dest string, autoPlace bool, placeType string) tea.Cmd {
	return m.downloadOrHoldCmd(ctx, urlStr, dest, true, autoPlace, placeType)
}

// downloadOrHoldCmd optionally starts download; when start=false it only probes and updates hold status

func (m *Model) downloadOrHoldCmd(ctx context.Context, urlStr, dest string, start bool, autoPlace bool, placeType string) tea.Cmd {
	return func() tea.Msg {
		resolved := urlStr
		headers := map[string]string{}
		normToast := ""
		// Translate civitai model pages and Hugging Face blob pages into resolver URIs for proper resolution
		if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
			if u, err := neturl.Parse(resolved); err == nil {
				h := strings.ToLower(u.Hostname())
				// CivitAI model page -> civitai://
				if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					if len(parts) >= 2 {
						modelID := parts[1]
						q := u.Query()
						ver := q.Get("modelVersionId")
						if ver == "" {
							ver = q.Get("version")
						}
						civ := "civitai://model/" + modelID
						if strings.TrimSpace(ver) != "" {
							civ += "?version=" + neturl.QueryEscape(ver)
						}
						normToast = "normalized CivitAI page → " + civ
						if res, err := resolver.Resolve(ctx, civ, m.cfg); err == nil {
							resolved = res.URL
							headers = res.Headers
							m.addToast(normToast)
						}
					}
				}
				// Hugging Face blob page -> hf://owner/repo/path?rev=...
				if hostIs(h, "huggingface.co") {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					if len(parts) >= 5 && parts[2] == "blob" {
						owner := parts[0]
						repo := parts[1]
						rev := parts[3]
						filePath := strings.Join(parts[4:], "/")
						hf := "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
						normToast = "normalized HF blob → " + hf
						if res, err := resolver.Resolve(ctx, hf, m.cfg); err == nil {
							resolved = res.URL
							headers = res.Headers
							m.addToast(normToast)
						}
					}
				}
			}
		}
		if strings.HasPrefix(resolved, "hf://") || strings.HasPrefix(resolved, "civitai://") {
			res, err := resolver.Resolve(ctx, resolved, m.cfg)
			if err != nil {
				return dlDoneMsg{url: urlStr, dest: dest, path: "", err: err, needsPlaceMap: false}
			}
			resolved = res.URL
			headers = res.Headers
		} else {
			if u, err := neturl.Parse(resolved); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "huggingface.co") && m.cfg.Sources.HuggingFace.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
					if env == "" {
						env = "HF_TOKEN"
					}
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					}
				}
				if hostIs(h, "civitai.com") && m.cfg.Sources.CivitAI.Enabled {
					env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
					if env == "" {
						env = "CIVITAI_TOKEN"
					}
					if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
						headers["Authorization"] = "Bearer " + tok
					}
				}
			}
		}
		// Merge any pre-inserted row under the original URL into the resolved URL key to avoid duplicates
		origKey := urlStr + "|" + dest
		resKey := resolved + "|" + dest
		// Use placement metadata passed as parameters (safe - no concurrent map access)
		needsMap := false
		autoPlaceVal := autoPlace
		placeTypeVal := placeType
		if origKey != resKey {
			// Mark that placement data needs to be remapped in Update()
			needsMap = true
			// Remove the old pending row and create/refresh the resolved one as pending
			_ = m.st.DeleteDownload(urlStr, dest)
			_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "pending"})
		}
		if start {
			if !m.cfg.Network.DisableAuthPreflight {
				// Quick reachability/auth probe; if network-unreachable or unauthorized, put job on hold and do not start download
				reach, info := downloader.CheckReachable(ctx, m.cfg, resolved, headers)
				if !reach {
					_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold", LastError: info})
					// Return placement data to Update() for safe map handling
					m.addToast("probe failed: unreachable; job on hold (" + info + ")")
					return dlDoneMsg{
						url: resolved, dest: dest, path: "", err: fmt.Errorf("hold: unreachable"),
						origKey: origKey, resKey: resKey, autoPlace: autoPlaceVal, placeType: placeTypeVal, needsPlaceMap: needsMap,
					}
				}
				// If reachable, parse status code for early auth guidance
				code := 0
				if sp := strings.Fields(info); len(sp) > 0 {
					_, _ = fmt.Sscanf(sp[0], "%d", &code)
				}
				if code == 401 || code == 403 {
					host := hostOf(resolved)
					msg := info
					if strings.HasSuffix(strings.ToLower(host), "huggingface.co") {
						env := strings.TrimSpace(m.cfg.Sources.HuggingFace.TokenEnv)
						if env == "" {
							env = "HF_TOKEN"
						}
						msg = fmt.Sprintf("%s — set %s and ensure repo access/license accepted", info, env)
					} else if strings.HasSuffix(strings.ToLower(host), "civitai.com") {
						env := strings.TrimSpace(m.cfg.Sources.CivitAI.TokenEnv)
						if env == "" {
							env = "CIVITAI_TOKEN"
						}
						msg = fmt.Sprintf("%s — set %s and ensure content is accessible", info, env)
					}
					_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold", LastError: msg})
					m.addToast("preflight auth: job on hold (" + msg + ")")
					return dlDoneMsg{
						url: resolved, dest: dest, path: "", err: fmt.Errorf("hold: unauthorized"),
						origKey: origKey, resKey: resKey, autoPlace: autoPlaceVal, placeType: placeTypeVal, needsPlaceMap: needsMap,
					}
				}
			}
			log := logging.New("error", false)
			dl := downloader.NewAuto(m.cfg, log, m.st, nil)
			final, _, err := dl.Download(ctx, resolved, dest, "", headers, m.cfg.General.AlwaysNoResume)
			return dlDoneMsg{
				url: urlStr, dest: dest, path: final, err: err,
				origKey: origKey, resKey: resKey, autoPlace: autoPlaceVal, placeType: placeTypeVal, needsPlaceMap: needsMap,
			}
		} else {
			// Probe-only path
			reach, info := downloader.CheckReachable(ctx, m.cfg, resolved, headers)
			if !reach {
				_ = m.st.UpsertDownload(state.DownloadRow{URL: resolved, Dest: dest, Status: "hold", LastError: info})
			}
			return probeMsg{url: resolved, dest: dest, reachable: reach, info: info}
		}
	}
}

// metadataStoredMsg is sent when metadata has been successfully fetched and stored
type metadataStoredMsg struct {
	url       string
	modelName string
}

// fetchAndStoreMetadataCmd returns a command that fetches and stores metadata in the background.
// This is fire-and-forget - errors are logged but don't affect the UI.
func (m *Model) fetchAndStoreMetadataCmd(url, dest, path string) tea.Cmd {
	if m.st == nil {
		return nil
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Create metadata registry
		registry := metadata.NewRegistry()

		// Fetch metadata from appropriate source
		meta, err := registry.FetchMetadata(ctx, url)
		if err != nil {
			// Log error but don't fail - metadata is optional
			if m.log != nil {
				m.log.Debugf("metadata fetch failed for %s: %v", url, err)
			}
			return nil
		}

		// Set destination path
		meta.DownloadURL = url
		meta.Dest = dest

		// Get file size if available
		if path != "" {
			if info, err := os.Stat(path); err == nil {
				meta.FileSize = info.Size()
			}
		}

		// Store metadata in database
		if err := m.st.UpsertMetadata(meta); err != nil {
			if m.log != nil {
				m.log.Debugf("metadata storage failed for %s: %v", url, err)
			}
			return nil
		}

		if m.log != nil {
			m.log.Debugf("stored metadata for %s (%s)", meta.ModelName, url)
		}

		return metadataStoredMsg{url: url, modelName: meta.ModelName}
	}
}
