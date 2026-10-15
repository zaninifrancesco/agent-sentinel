# Stato dell'implementazione

Branch: `telemetry-profiler`. Aggiornamento: 15 ottobre 2026.

## Fase 1 — Cattura passiva implementata

- `serve`, `mcp` e `hook` espongono `--mode observe`, predefinito. La CLI non costruisce policy, budget o broker di approvazione; il proxy osservativo non passa dal gate.
- `/api/config` dichiara `mode=observe`, `policy=off`, `approvals=false`, `budgetEditable=false`, `approvalTimeoutSec=0`.
- Gli hook restituiscono risposte neutrali anche offline, su errori HTTP e allo scadere del timeout. Il collector non può imporre al nuovo client hook una decisione di permesso, nemmeno se è una vecchia versione.
- Timeout HTTP di un secondo, configurabile fino a cinque; hook installati con `--mode observe` e un timeout di processo di sei secondi.
- Install/uninstall preservano gli hook e i settings degli altri strumenti. Le vecchie opzioni di enforcement vengono rifiutate con indicazioni di migrazione.
- Cockpit e report si concentrano sulle chiamate e sugli esiti; rimosse approvazioni, modifica dei budget e stime monetarie/token derivate da prezzi fissi.
- Le vecchie tracce restano leggibili, con decisioni storiche quando presenti. Gli eventi senza policy non mostrano una valutazione inventata.

## Cockpit — Primo profilo di analisi implementato

- Overview iniziale con attività aggregata per tool: chiamate, errori segnalati, mediana/p95 disponibili e JSON conservato.
- Classifiche delle cinque chiamate completate più lente e delle cinque risposte conservate più grandi. Ogni voce apre gli eventi da cui deriva; selezionare un tool filtra le sue chiamate.
- Calls con filtri per esito e tool, ricerca negli argomenti conservati e ordinamento per cattura, durata o dimensione della risposta. Event log separato per osservazioni di protocollo e ciclo di vita.
- Inspector compatto con argomenti/risultato in primo piano, JSON evento secondario e patch quando disponibile. Su mobile passaggio lista/dettaglio con ritorno, senza pannelli compressi.
- Navigazione da tastiera confinata alla lista attiva; focus contenuto nei dialoghi e ripristinato alla chiusura.
- Durate assenti conservate come sconosciute; zero esplicito rimane un campione valido. L’intervallo della registrazione deriva dai timestamp catturati e non cresce col tempo trascorso nel browser.

Queste aggregazioni sono calcolate nel frontend sui dati disponibili: anticipano l’ispezione del profiler, senza dichiarare completata la fase 2 del recorder o il motore di analisi Go.

## Verifiche riproducibili

- Test Go di CLI, neutralità degli adattatori, installazione e proxy con fake server: input sintetici prima bloccati vengono registrati senza decisioni Sentinel e inoltrati invariati nel proxy.
- Test del client hook contro collector chiuso, risposte legacy e server in stallo su header o body; nessuna attesa illimitata o override dei permessi di Claude Code.
- Test frontend di percentili, durata zero, esclusione di chiamate interrotte/RPC, byte UTF-8 conservati, journey senza policy, ranking/filtri, durate mancanti, intervallo catturato e HTML con contenuti ostili resi inerti.
- Smoke test locale con binario temporaneo: config osservativa e tre coppie sintetiche richiesta/risposta Cursor/Claude registrate con esito `ok` e senza policy. Nessun comando contenuto nei payload è stato eseguito.
- Ispezione del cockpit su desktop e viewport mobile da 390 px, inclusi stato vuoto, filtri senza risultati, classifica → dettaglio, errori e dialogo export con focus da tastiera. `profiler.png` mostra la nuova overview con 15 chiamate sintetiche (due errori e una aperta); `cockpit.png` resta la schermata storica. Nessun comando dei fixture viene eseguito.

Check di chiusura passati: `make test` (vet, race test, typecheck e otto test frontend) e `cd web && npm run build`. Verificati anche link locali e whitespace. Non sono benchmark di performance né una verifica live delle versioni correnti degli agenti esterni.

## Limiti ancora aperti

- Il recorder mantiene il modello precedente: una registrazione per processo, correlazione e metadati incompleti, possibili perdite dello stream non segnalate e scrittura del sink sincrona. La fase 2 risolve questi aspetti.
- `JSON retained` misura solo i payload conservati e riserializzati, non il traffico originale. Gli adattatori possono ridurre o omettere contenuti. Profili di redazione configurabili non ancora implementati.
- Metriche ancora calcolate nel frontend; il motore Go unico viene introdotto nelle fasi successive.
- Nessuna waterfall, analisi automatica delle ripetizioni, occupazione del contesto, usage nativo, diagnostica degli schemi o catalogo multi-sessione implementati in questa fase.
- Il package di policy e le sue regressioni interne restano nel repository, ma la CLI nuova non offre enforcement. La loro eliminazione non è un prerequisito per osservare passivamente.
- Il binario installato e gli hook reali dell'utente non sono stati cambiati. La migrazione di un'installazione esistente è un'operazione separata ed esplicita.
- Landing page invariata.

Prossimo passo: fase 2, iniziando da identità di registrazione/sessione/chiamata e qualità della correlazione, prima di aggiungere findings.
