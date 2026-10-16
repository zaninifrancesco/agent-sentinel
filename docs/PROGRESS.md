# Stato dell'implementazione

Branch: `telemetry-profiler`. Aggiornamento: 16 ottobre 2026.

## Primo MVP analytics — implementato

- Statistics contiene le analytics storiche; la home è ora Timeline su indicazione dell’utente: filtri 24h/7d/30d, agente e progetto; trend giornalieri UTC di chiamate/errori, classifiche per agente/progetto/tool e confronto del volume col periodo precedente.
- Catalogo ricostruibile dai JSONL in `~/.sentinel/sessions` e dal file scelto esplicitamente. Cache in memoria delle proiezioni senza payload, invalidata da dimensione/mtime; nessun database o dipendenza aggiunta. Copie identiche contate una volta, identità con copie diverse escluse con avviso.
- Elenco di sessioni native/connessioni con 25 righe per pagina UI; selezionare un giorno restringe l’elenco senza cambiare i totali del periodo. Registrazioni senza chiamate restano raggiungibili.
- Apertura di uno scope storico nelle API profiler/dettaglio/report tramite `recordingId`, senza sostituire il recorder attivo. Le risposte profiler dichiarano `readOnly`; il frontend non segue in polling le registrazioni storiche. Il report della registrazione selezionata resta disponibile.
- Metriche storiche Go: error rate `error/(ok+error)`, campioni e percentili nearest rank con zero valido, somma delle durate e JSON conservato. Giorni vuoti non provano inattività; timestamp assenti sono esclusi e dichiarati.
- Il recorder espone errori di scrittura e ultimo `seq` del prefisso scritto senza errore, senza promettere fsync. Il replay recupera un frammento JSON finale malformato solo a EOF, marcando `recoveredTail`; una riga completa corrotta rimane fatale. Nessuna riscrittura del log.
- Contratto e limiti in [ANALYTICS_API.md](ANALYTICS_API.md). Uso nativo/token/costi, findings, confronto A/B e heatmap annuale restano previsti.
- Verifiche: test di catalogo/duplicati/conflitti/riavvio/crescita, denominatori/filtri/finestre/sessioni senza chiamate, guasto del sink e recupero EOF; regressione di drill-down HTTP storico e conservazione del collector. `make test` e build frontend passati, dieci test frontend. Smoke browser con 12 registrazioni sintetiche/318 chiamate, filtri, apertura della sessione storica da 25 chiamate, desktop 1280×800 e mobile 390×844, chiaro/scuro. Nessun comando dei fixture eseguito. Nessun binario installato o hook reale modificato.

## Analisi delle chiamate come home

- Timeline è la vista iniziale e la prima voce di navigazione. Le statistiche generali sono nella vista dedicata Statistics; il grafico giornaliero è stato compattato.
- Waterfall integrata nell’elenco chiamate: una riga per tool con etichette leggibili, stato, argomenti, intervallo proporzionale e durata disponibile separata. Eliminata la duplicazione mappa/lista e la variante di layout dipendente dall’altezza del desktop.
- Ricerca, filtri, ordinamento e pagine restano disponibili. A larghezze inferiori a 1024px il dettaglio sostituisce la lista, con ritorno esplicito; zero, tempi assenti e fine mancante restano distinti. I quattro tasti freccia spostano la selezione nella waterfall.
- Test di rendering verificano etichette/selezione per chiamate brevi e senza timestamp, e separazione tra tempo catturato e durata tool. Le vecchie prove di packing delle tracce sono sostituite dai check del nuovo comportamento.

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
- Timeline come vista iniziale, con waterfall integrata e una riga etichettata per chiamata. La finestra segue la pagina corrente; sovrapposizioni e intervalli non attribuiti condividono l’asse temporale. Le durate riportate rimangono separate dagli intervalli catturati.
- Tool profile con attività aggregata per tool: chiamate, errori segnalati, mediana/p95 disponibili e JSON conservato.
- Classifiche delle cinque chiamate completate più lente e delle cinque risposte conservate più grandi. Ogni voce apre gli eventi da cui deriva; selezionare un tool filtra le sue chiamate.
- Calls con filtri per esito e tool, ricerca negli argomenti conservati e ordinamento per cattura, durata o dimensione della risposta. Event log separato per osservazioni di protocollo e ciclo di vita.
- Inspector con argomenti/risultato, evidenziazione sintattica sicura, ricerca testuale con conteggio dei risultati, navigazione precedente/successivo, JSON evento secondario e patch quando disponibile. Su mobile passaggio lista/dettaglio con ritorno, senza pannelli compressi.
- Navigazione da tastiera confinata alla lista attiva; focus contenuto nei dialoghi e ripristinato alla chiusura.
- Durate assenti conservate come sconosciute; zero esplicito rimane un campione valido. L’intervallo della registrazione deriva dai timestamp catturati e non cresce col tempo trascorso nel browser.

Le aggregazioni iniziali del frontend sono state sostituite dalla proiezione Go descritta sotto. Questo non chiude i lavori di affidabilità del recorder della fase 2.

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

Questa è una parte della fase 2: persistenza e stream restano da completare. Le diagnostiche di correlazione non sono il motore di findings R1–R4.

## Fase 2 — Profili e qualità dei payload implementati

- `serve` e `mcp` accettano `--capture standard|full` e `--max-payload-bytes N`. Standard è predefinito: 64 KiB per payload e redazione `structured-text-v1`; full è esplicito, senza redazione, con 1 MiB. Limite personalizzato tra 256 e 16.777.216 byte; zero usa il default del profilo. Le opzioni invalide vengono rifiutate prima di aprire log o trasporto.
- Redazione centrale prima di memoria, JSONL e subscriber live, seguita da un limite che conserva JSON valido e UTF-8. La copia inoltrata dal proxy MCP e il comando reale del server restano invariati. La redazione riconosce campi strutturati e pattern testuali: è best effort, non garantisce di trovare ogni segreto.
- Metadati `capture` separano il valore osservato dall’adattatore (campo e unità testo/JSON/frame), JSON normalizzato e JSON conservato. Le dimensioni dichiarate dal client hook restano distinte dalle misure del collector. Uno zero non diventa assenza; la dimensione originale non misurata resta sconosciuta.
- Completezza `complete|partial|omitted|unknown`, rappresentazione, profilo e trasformazioni versionate: normalizzazione, redazione, troncamento e omissione con stadio e quantità quando note. I limiti a monte restano dichiarati anche in full.
- Standard omette i contenuti delle letture Cursor e sintetizza alcune risposte Claude; full conserva il contenuto ricevuto e l’intero valore `tool_response` dei PostToolUse. Non implica la cattura di tutto il body hook o del transcript del modello.
- Metadati testuali sensibili riconosciuti vengono redatti; identità che corrispondono a pattern sensibili diventano HMAC opachi per registrazione, con campi modificati dichiarati. Il tracker mantiene correlazione e chiusura delle sessioni senza collassare ID distinti.
- Configurazione persistita all’inizio della registrazione e disponibile in sessione e `/api/config`; replay senza nuova redazione dei log esistenti. Inspector e patch mostrano qualità e misure con dettaglio espandibile; sidebar e report indicano il profilo. Le classifiche rimangono basate sul JSON conservato.

## Profiler — Waterfall, paginazione e metriche Go implementati

- `internal/analysis`, versione `profiler-v1`: proiezione delle chiamate v2 e legacy, scope, conteggi, percentili nearest rank, classifiche e misure temporali. Eliminati la seconda proiezione e i calcoli delle metriche dal browser. Durate sconosciute e zero espliciti restano distinti; attese storiche non separabili e interruzioni non entrano nei percentili.
- Timeline con finestra della pagina corrente: una riga per chiamata, etichette fisse con tool/argomenti e durata, intervalli proporzionali, zoom Fit/4×/16×/64× e selezione sincronizzata con l’inspector. Osservazioni puntuali coincidenti e chiamate senza timestamp restano separatamente ispezionabili; il numero di righe non è una misura di concorrenza. Copertura = unione degli intervalli tool abbinati; tempo non attribuito = complemento nell’intervallo osservato. Massima concorrenza su intervalli positivi, senza doppio conteggio del tempo trascorso.
- Incomplete, ambigue, osservazioni senza risposta e chiusure sintetiche non inventano una fine. Il confine dello snapshot deriva dalle evidenze conservate, senza crescita dipendente dall’orologio del browser. Nessuno span di ragionamento o turno LLM viene ricostruito.
- [API profiler v1](PROFILER_API.md): elenco di sintesi paginato (default 50, massimo 200), ricerca sugli argomenti conservati, filtri e ordinamento nel server. Cursori legati a registrazione/revisione/query, stabili quando arrivano nuovi eventi. Il dettaglio completo arriva solo per la chiamata selezionata.
- Metriche, profilo per tool e totali temporali seguono l’intero scope selezionato; filtri e pagine agiscono sull’elenco. Le classifiche e la ricerca aprono direttamente la pagina della chiamata. Stato di caricamento, errore e recupero con Refresh latest/Retry espliciti.
- Una sessione attiva omette `endedAt`, con regressione specifica contro la data zero serializzata. Prima pagina live aggiornata con richieste seriali ogni due secondi. Cursori fissati e registrazioni concluse si caricano una volta. Il cockpit non riceve più tutti i payload tramite lo snapshot WebSocket; le API precedenti restano compatibili.
- Report HTML con metriche, algoritmo, revisione e intervalli dello stesso snapshot Go, sempre sull’intera registrazione. Scarica tutte le sintesi solo su export esplicito e non i corpi dei payload; CSP ed escaping restano attivi.

## Verifiche riproducibili

- Test Go di CLI, neutralità degli adattatori, installazione e proxy con fake server: input sintetici prima bloccati vengono registrati senza decisioni Sentinel e inoltrati invariati nel proxy.
- Test del client hook contro collector chiuso, risposte legacy e server in stallo su header o body; nessuna attesa illimitata o override dei permessi di Claude Code.
- Le regressioni di proiezione/metriche sono ora in Go: nearest rank con zero, esclusioni, connessioni distinte, ambiguità, tempi invertiti, byte conservati, ordinamento e filtri. I test frontend verificano la presentazione dei valori del server, i journey storici e report con testo ostile reso inerte.
- Smoke test locale con binario temporaneo: config osservativa e tre coppie sintetiche richiesta/risposta Cursor/Claude registrate con esito `ok` e senza policy. Nessun comando contenuto nei payload è stato eseguito.
- Ispezione del cockpit su desktop 1440/1280 px e mobile da 390 px, nei temi chiaro/scuro, inclusi filtri senza risultati, mappa → chiamata → dettaglio, ricerca payload, precedente/successivo, classifica → dettaglio, errori e dialogo export con focus da tastiera. `profiler.png` mostra la nuova timeline Trace Atlas con 17 chiamate sintetiche (due errori e una aperta); `cockpit.png` resta la schermata storica. Nessun comando dei fixture viene eseguito.

- Nuove regressioni su sessioni con ID uguali, richieste MCP bidirezionali e su connessioni distinte, ID grandi, chiamate concorrenti, durate zero/assenti, risposte orfane e ambiguità legacy.
- Smoke test CLI con binario temporaneo: cattura HTTP di 18 eventi e sette richieste su tre scope nativi, due risposte ambigue, una orfana e una chiusura sintetica; replay JSONL riuscito. Fixture sanitizzata in `testdata/telemetry-v2.jsonl`. Controllati selettore, durata zero, filtro Unresolved, Event log e scope dell’export su desktop, dettaglio mobile a 390 px. Corretto il layout mobile del dettaglio dopo l’introduzione della sidebar richiudibile. Tutti i server temporanei vengono chiusi al termine.
- Regressioni di cattura: segreti sintetici assenti da JSONL e stream standard, redazione prima del limite, JSON valido entro soglia con Unicode e strutture grandi, numeri esatti, nessuna modifica dell’input del chiamante, full senza redazione e impostazioni ripristinate dal replay. Proxy con fake server che restituisce la richiesta ricevuta: frame inoltrati integri mentre la registrazione è ridotta/redatta.
- Smoke CLI standard/full su porte effimere: output sintetico UTF-8 da 9.036 byte ridotto a massimo 2.048 byte JSON, redazione e troncamento dichiarati, lettura omessa con dimensione riferita dal client separata, zero esplicito, tool response full conservata, profilo ripristinato nel replay. Controllati dettaglio espandibile desktop 1280×800 e mobile 390×844. Nessun comando nei payload eseguito; collector e replay temporanei chiusi.

Prova del profiler del 16 ottobre: fixture temporanea con 10.000 richieste, 9.998 risposte, due connessioni, 323 errori e due chiamate senza risposta. Verificati cambio pagina, salto da classifica alla pagina corretta, scope, filtro Open, dettaglio mobile a 390 px ed export HTML reale con 10.000 chiamate anche da una vista filtrata. Nessun comando dei mock eseguito.

Nel collector live su 8848 è stato verificato anche il passaggio da 64 a 65 chiamate via polling, senza reload del browser, con un errore, una chiamata aperta e due intervalli sovrapposti. Gli eventi sono sintetici e i comandi restano solo stringhe.

Ambiente della prova: macOS arm64, Go 1.27.1, Node 26.4.0. Una prima risposta HTTP locale da 50 sintesi pesava 109.877 byte e ha richiesto circa 53 ms nel singolo smoke; RSS del processo replay dopo la verifica circa 46,2 MiB (47.328 KiB), escluso il browser. Sono osservazioni di questa fixture sintetica, non benchmark o limiti garantiti. Nel browser sono state ispezionate le viste laptop 1280×800 e mobile 390×844; il trasferimento ordinario non include i 10.000 payload.

Revisione del redesign: il problema materiale dell’altezza del payload sui laptop è stato corretto e verificato dal revisore come risolto.

Check di chiusura passati: `make test` (vet, race test, typecheck e nove test frontend; proiezione e calcoli spostati nei test Go) e `cd web && npm run build`. Detector UI senza segnalazioni. Verificati anche link locali e whitespace. Non sono benchmark di performance né una verifica live delle versioni correnti degli agenti esterni.

## Limiti ancora aperti

- Il recorder mantiene in memoria l’intera registrazione e scrive il sink in modo sincrono. Code limitate, gestione dei gap dello stream e recupero del sink restano aperti nella fase 2. Gli errori del sink sono ora visibili e la coda JSONL incompleta è recuperabile nel caso EOF dichiarato. La paginazione v1 limita il trasferimento e la memoria ordinaria del browser; non limita la memoria totale del recorder o la dimensione di ogni riga del dettaglio.
- `JSON retained` misura il JSON conservato di richieste e risposte abbinate nelle chiamate visualizzate, non il traffico originale; per i log precedenti usa il JSON conservato compatto. Gli adattatori possono ridurre o omettere contenuti. Le dimensioni della sorgente non ricostruiscono ciò che non è arrivato al punto di cattura. Il limite per payload non limita envelope, input, code o memoria totale della registrazione.
- La waterfall mostra gli intervalli catturati dei tool, senza span nativi di modello o turno. I suoi intervalli non attribuiti non sono una diagnostica di perdita di eventi. Nessuna analisi automatica delle ripetizioni, occupazione del contesto, usage nativo o diagnostica degli schemi implementata. Catalogo e trend multi-sessione sono disponibili nel MVP analytics; la cache conserva le sintesi di tutte le registrazioni e non ha ancora un limite totale.
- Il package di policy e le sue regressioni interne restano nel repository, ma la CLI nuova non offre enforcement. La loro eliminazione non è un prerequisito per osservare passivamente.
- Il binario installato e gli hook reali dell'utente non sono stati cambiati. La migrazione di un'installazione esistente è un'operazione separata ed esplicita.
- Landing page invariata.

Prossimo passo di prodotto: diagnostica R1–R4 e analisi dei volumi con evidenze, poi usage nativo e confronto A/B. Proseguire anche con capability, code e memoria limitate e gap distinguibili. Il primo MVP di catalogo/dashboard/trend è disponibile.
