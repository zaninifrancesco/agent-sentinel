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

## Cockpit — Trace Atlas implementato

- Nuova identità Trace Atlas: navigazione grafite richiudibile, superfici fredde, colori per tool e selezione sincronizzata. Sui desktop bassi la mappa resta sopra la lista e l’inspector usa tutta l’altezza disponibile.
- Timeline come vista iniziale, con mappa selezionabile per tool. Gli intervalli derivano dai timestamp catturati, separati dalla durata riportata; risposte senza timestamp valido restano punti. Ultime 120 chiamate nella mappa, tutte nella lista.
- Tool profile con attività aggregata per tool: chiamate, errori segnalati, mediana/p95 disponibili e JSON conservato.
- Classifiche delle cinque chiamate completate più lente e delle cinque risposte conservate più grandi. Ogni voce apre gli eventi da cui deriva; selezionare un tool filtra le sue chiamate.
- Calls con filtri per esito e tool, ricerca negli argomenti conservati e ordinamento per cattura, durata o dimensione della risposta. Event log separato per osservazioni di protocollo e ciclo di vita.
- Inspector con argomenti/risultato, evidenziazione sintattica sicura, ricerca testuale con conteggio dei risultati, navigazione precedente/successivo, JSON evento secondario e patch quando disponibile. Su mobile passaggio lista/dettaglio con ritorno, senza pannelli compressi.
- Navigazione da tastiera confinata alla lista attiva; focus contenuto nei dialoghi e ripristinato alla chiusura.
- Durate assenti conservate come sconosciute; zero esplicito rimane un campione valido. L’intervallo della registrazione deriva dai timestamp catturati e non cresce col tempo trascorso nel browser.

Queste aggregazioni sono calcolate nel frontend sui dati disponibili: anticipano l’ispezione del profiler, senza dichiarare completata la fase 2 del recorder o il motore di analisi Go.

## Fase 2 — Identità e correlazione implementate

- Nuove scritture `schemaVersion: 2` con `recordingId`; `sessionId` rimane un alias della registrazione per compatibilità, distinto da `agentSessionId` nativo.
- Sorgenti hook distinte per agente e namespace locale del collector; connessione propria per ogni proxy MCP. `callId` stabile nel JSONL e riferimenti alla richiesta abbinata.
- ID nativi tipizzati con valore testuale, così un numero grande non perde precisione nel browser e una stringa non coincide con un numero.
- Claude usa il tool-use ID quando presente. Il fallback degli hook richiede una sessione nota e confronta tool/server e argomenti canonici conservando la precisione numerica; non usa un payload già troncato come identità.
- Il tracker comune distingue connessione e lato originatore delle richieste MCP. Chiamate concorrenti indistinguibili generano diagnostica con gli ID candidati; nessuna risposta viene attribuita in FIFO. L’intero gruppo resta ambiguo fino al suo esaurimento.
- Risposte senza richiesta conservate come osservazioni parziali. Chiusure sintetiche di un turno/sessione marcate esplicitamente, senza inventare una durata di esecuzione.
- `timing` conserva zero e provenienza (`source-reported`, se trasmessa, oppure `proxy-monotonic`). Un campo mancante resta sconosciuto.
- Selettore di sessione/connessione nel cockpit; mappa, profilo, lista e metriche seguono lo scope. Filtro Unresolved, dettagli di correlazione, risposte non abbinate nell’Event log e report con identità/qualità. L’export include l’intera registrazione anche con un filtro attivo.
- Replay dei log precedenti senza riscritture; tipi di evento sconosciuti restano ispezionabili. Major di schema non supportate, identità di registrazione miste e sequenze v2 non crescenti vengono rifiutate.

Questa è una parte della fase 2: le garanzie su payload, persistenza e stream restano da completare. Le diagnostiche di correlazione non sono il motore di findings R1–R4.

## Verifiche riproducibili

- Test Go di CLI, neutralità degli adattatori, installazione e proxy con fake server: input sintetici prima bloccati vengono registrati senza decisioni Sentinel e inoltrati invariati nel proxy.
- Test del client hook contro collector chiuso, risposte legacy e server in stallo su header o body; nessuna attesa illimitata o override dei permessi di Claude Code.
- Test frontend di percentili, durata zero, esclusione di chiamate interrotte/RPC, byte UTF-8 conservati, journey senza policy, ranking/filtri, durate mancanti, intervallo catturato e HTML con contenuti ostili resi inerti.
- Smoke test locale con binario temporaneo: config osservativa e tre coppie sintetiche richiesta/risposta Cursor/Claude registrate con esito `ok` e senza policy. Nessun comando contenuto nei payload è stato eseguito.
- Ispezione del cockpit su desktop 1440/1280 px e mobile da 390 px, nei temi chiaro/scuro, inclusi filtri senza risultati, mappa → chiamata → dettaglio, ricerca payload, precedente/successivo, classifica → dettaglio, errori e dialogo export con focus da tastiera. `profiler.png` mostra la nuova timeline Trace Atlas con 17 chiamate sintetiche (due errori e una aperta); `cockpit.png` resta la schermata storica. Nessun comando dei fixture viene eseguito.

- Nuove regressioni su sessioni con ID uguali, richieste MCP bidirezionali e su connessioni distinte, ID grandi, chiamate concorrenti, durate zero/assenti, risposte orfane e ambiguità legacy.
- Smoke test CLI con binario temporaneo: cattura HTTP di 18 eventi e sette richieste su tre scope nativi, due risposte ambigue, una orfana e una chiusura sintetica; replay JSONL riuscito. Fixture sanitizzata in `testdata/telemetry-v2.jsonl`. Controllati selettore, durata zero, filtro Unresolved, Event log e scope dell’export su desktop, dettaglio mobile a 390 px. Corretto il layout mobile del dettaglio dopo l’introduzione della sidebar richiudibile. Tutti i server temporanei vengono chiusi al termine.

Revisione del redesign: il problema materiale dell’altezza del payload sui laptop è stato corretto e verificato dal revisore come risolto.

Check di chiusura passati: `make test` (vet, race test, typecheck e diciassette test frontend) e `cd web && npm run build`. Verificati anche link locali e whitespace. Non sono benchmark di performance né una verifica live delle versioni correnti degli agenti esterni.

## Limiti ancora aperti

- Il recorder mantiene in memoria l’intera registrazione e scrive il sink in modo sincrono. Code limitate, errori del sink, gap dello stream e recupero della coda incompleta restano aperti nella fase 2. Gli endpoint HTTP correnti continuano a servire una registrazione per processo, senza paginazione.
- `JSON retained` misura solo i payload conservati e riserializzati, non il traffico originale. Gli adattatori possono ridurre o omettere contenuti. Profili di redazione configurabili non ancora implementati.
- Metriche ancora calcolate nel frontend; il motore Go unico viene introdotto nelle fasi successive.
- La mappa mostra soltanto intervalli di cattura, non una waterfall completa di turni LLM/tool. Nessuna analisi automatica delle ripetizioni, occupazione del contesto, usage nativo, diagnostica degli schemi o catalogo multi-sessione implementati in questa fase.
- Il package di policy e le sue regressioni interne restano nel repository, ma la CLI nuova non offre enforcement. La loro eliminazione non è un prerequisito per osservare passivamente.
- Il binario installato e gli hook reali dell'utente non sono stati cambiati. La migrazione di un'installazione esistente è un'operazione separata ed esplicita.
- Landing page invariata.

Prossimo passo: completare la qualità della cattura nella fase 2, iniziando da dimensioni e completezza dei payload e redazione prima della persistenza, quindi code limitate, gap e API paginata prima dei findings.
