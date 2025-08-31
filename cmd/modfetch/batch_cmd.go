package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"modfetch/internal/batch"
	"modfetch/internal/config"
	"modfetch/internal/downloader"
	"modfetch/internal/logging"
	"modfetch/internal/resolver"
	"modfetch/internal/util"
)

func handleBatch(args []string) error {
	if len(args) == 0 { return errors.New("batch subcommand required: import") }
	sub := args[0]
	switch sub {
	case "import":
		return handleBatchImport(args[1:])
	default:
		return fmt.Errorf("unknown batch subcommand: %s", sub)
	}
}

func handleBatchImport(args []string) error {
	fs := flag.NewFlagSet("batch import", flag.ContinueOnError)
	cfgPath := fs.String("config", "", "Path to YAML config file")
	logLevel := fs.String("log-level", "info", "log level")
	jsonOut := fs.Bool("json", false, "json logs")
	inPath := fs.String("input", "", "Text file with URLs (one per line). Supports optional key=value pairs after the URL: dest=... sha256=... type=... place=true mode=symlink")
	outPath := fs.String("output", "", "Output batch YAML path (default: stdout)")
	destDir := fs.String("dest-dir", "", "Destination directory override (default: config.general.download_root)")
	shaMode := fs.String("sha-mode", "none", "SHA mode: none|compute (compute downloads content to hash)")
	defType := fs.String("type", "", "Default artifact type for jobs (override per-line via type=)")
	defPlace := fs.Bool("place", false, "Default place flag for jobs (override per-line via place=")
	defMode := fs.String("mode", "", "Default placement mode: symlink|hardlink|copy (override per-line via mode=")
	noResolvePages := fs.Bool("no-resolve-pages", false, "Disable civitai.com model page -> civitai:// uri normalization")
	if err := fs.Parse(args); err != nil { return err }
	if *cfgPath == "" { if env := os.Getenv("MODFETCH_CONFIG"); env != "" { *cfgPath = env } }
	if *cfgPath == "" { return errors.New("--config is required or set MODFETCH_CONFIG") }
	if *inPath == "" { return errors.New("--input is required") }
	c, err := config.Load(*cfgPath)
	if err != nil { return err }
	log := logging.New(*logLevel, *jsonOut)

	f, err := os.Open(*inPath)
	if err != nil { return err }
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 1024), 1024*1024)

	// defaults
	out := &batch.File{Version: 1}
	ctx := context.Background()
	root := strings.TrimSpace(*destDir)
	if root == "" { root = c.General.DownloadRoot }
	if err := os.MkdirAll(root, 0o755); err != nil { return err }

	lineNum := 0
	for s.Scan() {
		lineNum++
		line := strings.TrimSpace(s.Text())
		if line == "" { continue }
		if strings.HasPrefix(strings.TrimSpace(line), "#") { continue }
		// Split into tokens
		toks := strings.Fields(line)
		if len(toks) == 0 { continue }
		uri := toks[0]
		jDest := ""
		jSHA := ""
		jType := strings.TrimSpace(*defType)
		jPlace := *defPlace
		jMode := strings.TrimSpace(*defMode)
		for _, t := range toks[1:] {
			if !strings.Contains(t, "=") { continue }
			kv := strings.SplitN(t, "=", 2)
			k := strings.ToLower(strings.TrimSpace(kv[0]))
			v := ""; if len(kv) > 1 { v = strings.TrimSpace(kv[1]) }
			switch k {
			case "dest": jDest = v
			case "sha256": jSHA = v
			case "type": jType = v
			case "place": jPlace = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "yes")
			case "mode": jMode = v
			}
		}

		// Normalize page URLs -> resolver URIs (optional)
		resolvedURI := uri
		if !*noResolvePages && (strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://")) {
			if u, err := neturl.Parse(uri); err == nil {
				h := strings.ToLower(u.Hostname())
				// CivitAI model page -> civitai://
				if hostIs(h, "civitai.com") && strings.HasPrefix(u.Path, "/models/") {
					parts := strings.Split(strings.Trim(u.Path, "/"), "/")
					if len(parts) >= 2 {
						modelID := parts[1]
						q := u.Query()
						ver := q.Get("modelVersionId"); if ver == "" { ver = q.Get("version") }
						civ := "civitai://model/" + modelID
						if strings.TrimSpace(ver) != "" { civ += "?version=" + ver }
						resolvedURI = civ
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
						resolvedURI = "hf://" + owner + "/" + repo + "/" + filePath + "?rev=" + rev
					}
				}
			}
		}

		// Resolve headers and direct URL for probing
		headers := map[string]string{}
		probeURL := resolvedURI
		// If resolver URI, resolve to direct URL and headers
		if strings.HasPrefix(resolvedURI, "hf://") || strings.HasPrefix(resolvedURI, "civitai://") {
			// Warn if token is configured but not present in env
			if strings.HasPrefix(resolvedURI, "civitai://") && c.Sources.CivitAI.Enabled {
				if env := strings.TrimSpace(c.Sources.CivitAI.TokenEnv); env != "" {
					if tok := strings.TrimSpace(os.Getenv(env)); tok == "" {
						log.Warnf("CivitAI token env %s is not set; gated content will return 401. Export %s.", env, env)
					}
				}
			}
			if strings.HasPrefix(resolvedURI, "hf://") && c.Sources.HuggingFace.Enabled {
				if env := strings.TrimSpace(c.Sources.HuggingFace.TokenEnv); env != "" {
					if tok := strings.TrimSpace(os.Getenv(env)); tok == "" {
						log.Warnf("Hugging Face token env %s is not set; gated repos will return 401. Export %s and accept the repo license.", env, env)
					}
				}
			}
			res, err := resolver.Resolve(ctx, resolvedURI, c)
			if err != nil { return fmt.Errorf("line %d: resolve: %w", lineNum, err) }
			probeURL = res.URL
			headers = res.Headers
			// For civitai, if no dest given, prefer SuggestedFilename with version hint
			if jDest == "" && strings.HasPrefix(resolvedURI, "civitai://") && strings.TrimSpace(res.SuggestedFilename) != "" {
				if p, err := util.UniquePath(root, res.SuggestedFilename, res.VersionID); err == nil { jDest = p }
			}
		} else {
			// Attach auth headers for known hosts (direct URLs)
			if u, err := neturl.Parse(probeURL); err == nil {
				h := strings.ToLower(u.Hostname())
				if hostIs(h, "civitai.com") && c.Sources.CivitAI.Enabled {
					if env := strings.TrimSpace(c.Sources.CivitAI.TokenEnv); env != "" {
						if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
							headers["Authorization"] = "Bearer " + tok
						} else {
							log.Warnf("CivitAI token env %s is not set; gated content will return 401. Export %s.", env, env)
						}
					}
				}
				if hostIs(h, "huggingface.co") && c.Sources.HuggingFace.Enabled {
					if env := strings.TrimSpace(c.Sources.HuggingFace.TokenEnv); env != "" {
						if tok := strings.TrimSpace(os.Getenv(env)); tok != "" {
							headers["Authorization"] = "Bearer " + tok
						} else {
							log.Warnf("Hugging Face token env %s is not set; gated repos will return 401. Export %s and accept the repo license.", env, env)
						}
					}
				}
			}
		}

		// Probe for filename and final URL
		meta, err := downloader.ProbeURL(ctx, c, probeURL, headers)
		if err != nil {
			log.Warnf("line %d: probe failed for %s: %v", lineNum, probeURL, err)
		}
		// If no dest provided yet, choose a good one
		if strings.TrimSpace(jDest) == "" {
			name := strings.TrimSpace(meta.Filename)
			if name == "" {
				base := meta.FinalURL
				if base == "" { base = probeURL }
				name = filepath.Base(base)
			}
			name = util.SafeFileName(name)
			p, err := util.UniquePath(root, name, "")
			if err != nil { return fmt.Errorf("line %d: dest compute: %w", lineNum, err) }
			jDest = p
		}

		// Maybe compute SHA256 by streaming; this is heavy
		if strings.EqualFold(strings.TrimSpace(*shaMode), "compute") && strings.TrimSpace(jSHA) == "" {
			log.Infof("computing sha256 for %s (this may take a while)", uri)
			sha, err := downloader.ComputeRemoteSHA256(ctx, c, probeURL, headers)
			if err != nil { return fmt.Errorf("line %d: compute sha: %w", lineNum, err) }
			jSHA = sha
		}

		out.Jobs = append(out.Jobs, batch.BatchJob{URI: resolvedURI, Dest: jDest, SHA256: jSHA, Type: jType, Place: jPlace, Mode: jMode})
	}
	if err := s.Err(); err != nil { return err }

	// Emit YAML
	if strings.TrimSpace(*outPath) == "" {
		// stdout
		enc, _ := yamlEncoder()
		if err := enc.Encode(out); err != nil { return err }
		return nil
	}
	if err := batch.Save(*outPath, out); err != nil { return err }
	fmt.Fprintf(os.Stderr, "wrote batch: %s (%d jobs)\n", *outPath, len(out.Jobs))
	return nil
}

// yamlEncoder provides a YAML encoder to stdout with indentation settings.
func yamlEncoder() (*yaml.Encoder, error) {
	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	return enc, nil
}

