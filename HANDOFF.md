# HANDOFF.md — Agent Sentinel

> **Data & Ora:** 2026-10-08 19:45 (Local Time)  
> **Repository:** `/Users/francescozanini/Desktop/dev/agent-sentinel`  
> **Remote Git:** `https://github.com/zaninifrancesco/agent-sentinel.git`

---

## 1. Obiettivo del Task Corrente
* **Missione:** Analisi strategica di 5 repository virali (`t3code`, `hub`, `ai-hedge-fund`, `docker-agent`, `busbar`) per individuare un progetto originale, ad alto impatto su CV (Hiring-Ready per SWE 23 anni) e ad alto potenziale di trazione nella community AI/DevTools.
* **Progetto Selezionato:** **Agent Sentinel** — Un flight recorder, execution boundary e cockpit locale (single binary) per AI coding agent (Claude Code, Aider, Cursor, Codex).
* **Fase Attuale:** Conclusa la fase di ideazione e setup dell'ambiente di sviluppo; avviata la **Milestone 1**.

---

## 2. Decisioni Architetturali Prese

1. **Linguaggio Core Backend — Go (Golang 1.27):**
   * *Perché:* Compilazione in singolo binario statico a zero dipendenze; gestione nativa e leggera di I/O concorrente con Goroutine e canali; controllo a basso livello di processi del sistema operativo (PTY/segnali); forte differenziatore nei colloqui tecnici rispetto al sovraffollamento di progetti in Python.
2. **Distribuzione Cockpit UI — `go:embed`:**
   * La dashboard web (React + Vite + TailwindCSS + Lucide/Shadcn) verrà compilata e incorporata direttamente nell'eseguibile Go. L'utente finale scarica un solo binario ed esegue `./sentinel` senza dover installare Node.js, Python o container.
3. **Protocolli Standard di Comunicazione:**
   * **JSON-RPC 2.0 / MCP (Model Context Protocol):** standard de facto per l'intercettazione non invasiva di tool calls (`tools/call`, `resources/read`).
   * **WebSockets bidirezionali:** per lo streaming a bassa latenza di log ed eventi verso il cockpit e per la gestione istantanea delle approvazioni (Human-in-the-Loop).
4. **Security & Boundary Model:**
   * 4 livelli di enforcement: `ALLOW`, `WARN`, `REQUIRE_APPROVAL` (pausa del processo con modale nel browser), `BLOCK` (rifiuto immediato per prevenzione leak di `.env` / chiavi SSH).
   * Token and cost circuit breaker per sessione.
5. **Standalone Report Generator:**
   * Esportazione a fine sessione di un file `audit-report.html` auto-contenuto (CSS/JS inline, diagrammi Mermaid, diff visuali) allegabile a Pull Request e issue GitHub.

---

## 3. File Modificati e Creati

Nel repository `agent-sentinel`:
* [`README.md`](file:///Users/francescozanini/Desktop/dev/agent-sentinel/README.md): Manifesto pubblico, problem statement, diagramma architetturale ASCII, preview della CLI UX e roadmap delle 5 milestone.
* [`docs/ARCHITECTURE.md`](file:///Users/francescozanini/Desktop/dev/agent-sentinel/docs/ARCHITECTURE.md): Specifica architetturale completa con diagrammi Mermaid, analisi delle target personas, specifiche tecniche dei moduli e data schema degli eventi.
* [`.gitignore`](file:///Users/francescozanini/Desktop/dev/agent-sentinel/.gitignore): Configurato per escludere binari Go compilati, artefatti macOS (`.DS_Store`) e build frontend.
* [`go.mod`](file:///Users/francescozanini/Desktop/dev/agent-sentinel/go.mod): Inizializzato per il modulo `github.com/zaninifrancesco/agent-sentinel`.
* [`cmd/sentinel/main.go`](file:///Users/francescozanini/Desktop/dev/agent-sentinel/cmd/sentinel/main.go): Entrypoint CLI funzionante con routing dei comandi (`run`, `mcp`, `ui`, `version`, `help`) e banner grafico.
* Alberatura cartelle creata:
  * `cmd/sentinel/`
  * `internal/proxy/`
  * `internal/policy/`
  * `internal/recorder/`
  * `internal/server/`
  * `web/`

*Primo commit effettuato sul ramo `master`:* `e36a024`.

---

## 4. Problemi Ancora Aperti & Note Tecniche

* **MCP Proxy Streaming:** Occorre definire come gestire l'intercettazione di stream `stdio` bidirezionale (stdin/stdout) senza introdurre latenza percettibile né corrompere i frame JSON-RPC quando l'agente emette output misto (testo libero + oggetti JSON-RPC).
* **Cross-Platform PTY:** Per il comando `sentinel run <agent>` su macOS/Linux servirà una libreria PTY pura (come `github.com/creack/pty`) con gestione pulita delle dimensioni della finestra terminale (SIGWINCH).
* **Setup Frontend:** La cartella `web/` non è ancora stata inizializzata con Vite/React; verrà fatto nella Milestone 2.

---

## 5. Prossimo Step Esatto da Implementare

👉 **Milestone 1 — Implementazione dei tipi di protocollo e dell'intercettore MCP/JSON-RPC:**
1. Creare il package `internal/protocol/`:
   * Strutture dati Go per messaggi JSON-RPC 2.0 (`Request`, `Response`, `Error`).
   * Strutture dati per le chiamate MCP (`CallToolRequest`, `CallToolResult`, `ListToolsResult`).
2. Creare il package `internal/recorder/`:
   * Schema dell'evento canonico (`Event`, `Session`, `RiskLevel`, `Status`).
3. Creare il parser di stream in `internal/proxy/`:
   * Scanner/Reader su `io.Reader` in grado di fare sniffing non bloccante dei frame JSON-RPC da `stdin`/`stdout`.
