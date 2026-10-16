# Analytics API v1

Implementata sul branch `telemetry-profiler`, 16 ottobre 2026. Il contratto storico usa la stessa proiezione Go delle chiamate del [profiler](PROFILER_API.md). Non espone token, costi o occupazione del contesto inferiti dai payload.

## Lettura dello storico

`GET /api/v1/analytics?range=7d&agent=cursor-hook&project=...`

- `range`: `24h`, `7d` (default), `30d`; intervallo mobile `[from,to)`.
- `to`: RFC3339 opzionale; default ora UTC del server. Consente confronti riproducibili sulla stessa selezione temporale, purché l’archivio non cambi.
- `agent`, `project`: filtri esatti sui metadati disponibili; stringa vuota = tutti. `unknown` raggruppa il dato assente.
- `day`: `YYYY-MM-DD` UTC opzionale; restringe soltanto `sessions`, mantenendo totali, trend e breakdown del periodo.
- Periodo, data o giorno malformati: 400.

La risposta contiene `version=analytics-v1`, `from`, `to`, `timezone=UTC`, `totals`, `previous`, `days`, `agents`, `projects`, `tools`, `availableAgents`, `availableProjects`, `sessions`, `recordingCount`, `nativeSessions`, `undatedCalls`, `warnings`, `persistence` e `currentRecordingId`.

`totals` e ogni gruppo hanno chiamate, successi/errori riportati, aperte, interrotte e altri esiti; `errorSamples=ok+errors`, `errorRate` nullo senza campioni; durate valide con `latencySamples`, `p50` e `p95` nearest rank (nulli senza campioni, zero valido); `toolDurationMs` cumulativo e `retainedBytes`. Le misure dei byte sono JSON conservato di richieste e risposte abbinate. Intervalli paralleli possono rendere il tempo cumulativo superiore a quello trascorso.

`previous` usa gli stessi filtri e un intervallo precedente di pari durata. Non si fa la media dei percentili dei file. I bucket giornalieri comprendono anche giorni senza chiamate presenti nell’archivio: non provano che l’agente fosse inattivo o che la cattura fosse completa. L’attribuzione temporale usa l’inizio/osservazione catturato della chiamata; timestamp assenti sono esclusi e contati in `undatedCalls` dopo i filtri agente/progetto.

## Catalogo e sessioni

Il catalogo legge i file regolari `.jsonl` direttamente sotto `~/.sentinel/sessions` e il file scelto esplicitamente da `ui` o `--log`. Non esegue scansioni ricorsive e non segue i symlink dell’archivio. Directory personalizzate diverse da quella predefinita non sono autoindicizzate. Il recorder corrente è incluso anche senza file e prevale sulla propria copia su disco.

La cache di proiezioni senza payload è invalidata da dimensione/mtime; i JSONL sono la fonte persistente, con indice ricostruito al riavvio. Copie byte-identiche con la stessa identità contano una volta; copie diverse con la stessa identità sono escluse con avviso. Log invalidi restano su disco e producono un avviso. Un’ultima riga malformata senza newline a EOF può essere recuperata se esiste già un prefisso valido: `recoveredTail` rende visibile l’incompletezza. Corruzione in una riga completa resta un errore.

Ogni voce `sessions` descrive uno scope nella registrazione: `recordingId`, `scope`, `agent`, `nativeSessionId` quando noto, `projects`, primo/ultimo timestamp delle chiamate selezionate, `live`, `ended`, `recoveredTail` e metriche. Senza tool viene esposta una voce di registrazione con metriche senza campioni, nei limiti temporali del suo inizio. Uno scope senza sessione nativa non aumenta `nativeSessions`; sessioni omonime in registrazioni diverse non vengono fuse. `live` indica il collector corrente, non dimostra che una specifica conversazione sia ancora attiva.

La risposta include tutte le sintesi di sessione della selezione; il cockpit ne rende 25 per pagina. Questa è paginazione UI, distinta dalla paginazione server delle chiamate del profiler. Le opzioni agente/progetto riflettono i metadati delle chiamate nell’archivio disponibile, indipendentemente dai filtri selezionati.

## Apertura delle evidenze

Gli endpoint profiler, riga e report accettano `recordingId` del catalogo. L’identità viene risolta solo tra i file indicizzati, mai come un percorso fornito dal browser. Le pagine profiler dichiarano `readOnly=true` per lo storico e per il processo replay. Cursori e revisioni restano fissati alla registrazione selezionata. Una registrazione non disponibile o cambiata produce 409; una riga assente nello snapshot produce 404.

Aprire una sessione dalla dashboard filtra il profiler per il suo scope e mostra tutte le chiamate dello scope, non soltanto quelle nel periodo analytics. Il collector e l’endpoint hook rimangono legati al recorder originario. Il report esporta l’intera registrazione selezionata, come prima.

## Persistenza e limiti

`persistence` descrive il recorder corrente: `enabled`, `writtenThrough` e primo/ultimo errore registrato in `error` quando presente. `writtenThrough` si ferma all’ultimo evento del prefisso scritto senza errori; non dichiara fsync o durabilità dopo un crash. In replay non esiste un writer attivo. Una lettura riuscita della memoria non prova che il log sia completo su disco.

Il sink resta sincrono e la memoria totale non è ancora limitata. La prima indicizzazione legge ogni file; la cache mantiene sintesi per tutte le chiamate. I dettagli delle registrazioni storiche vengono caricati su richiesta e possono richiedere la lettura del relativo JSONL. Questo MVP non introduce code asincrone, retention, recupero dei gap, polling universale degli agenti, usage nativo o confronti A/B.
