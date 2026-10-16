# Profiler HTTP v1

Implementato sul branch `telemetry-profiler`, 16 ottobre 2026. Versione degli algoritmi: `profiler-v1`, separata dallo schema degli eventi v2. Tutti gli endpoint sono locali e di sola lettura, con le difese Host/Origin già presenti. Servono una registrazione per processo.

## Pagina e snapshot

`GET /api/v1/profiler`

Parametri:

- `view=calls|events`, default `calls`. Calls include le richieste tool osservate; Events conserva anche protocollo, risposte non assegnate e ciclo di vita.
- `scope`: ID restituito in `scopes`; assente seleziona tutta la registrazione.
- `filter=all|errors|open|interrupted|unresolved`, `tool`, `q`, `sort=time|duration|response`. I filtri per stato/tool si applicano a Calls. La ricerca Calls comprende gli argomenti conservati completi; Events cerca titolo e sintesi.
- `limit`: 1–200, default 50. Il cockpit usa 50.
- `cursor`: valore opaco di `current`, `next` o `previous` della risposta. Fissa registrazione, revisione, posizione e firma della query. Non riutilizzarlo con filtri o limite diversi.
- `through`: revisione esplicita per un prefisso della registrazione. Una revisione futura viene rifiutata.
- `focus`: chiave della riga da individuare nell’insieme filtrato; apre la pagina contenente quella riga, usato dalle classifiche e dalla ricerca.
- `recordingId`: controllo opzionale dell’identità, utile per riconoscere un riavvio del collector.

Risposta: `version`, `revision`, `session`, `scopes`, `metrics`, `profile`, `timeline`, `rows`, `total`, `offset`, `limit`, `current`, `next` e `previous` quando disponibili. Le righe della pagina e delle classifiche sono sintesi: conservano identità, metadati degli eventi, qualità, `requestBytes`, `responseBytes`, `durationMs`, `toolDurationMs` e `span`, senza `payload`.

Metriche, profilo per tool e totali temporali descrivono **tutto lo scope selezionato**, indipendentemente da filtri, ordinamento o pagina della lista. `total` è invece il conteggio delle righe filtrate. Una nuova risposta non chiude retroattivamente una richiesta nello snapshot di un cursore precedente.

Query/cursori invalidi restituiscono 400; registrazione cambiata o revisione futura 409. Il cockpit propone Refresh latest per tornare a uno snapshot corrente. Nessun token di autorizzazione o contenuto segreto viene inserito nel cursore.

## Dettaglio ed export

`GET /api/v1/rows/{key}?through=N&recordingId=ID`

Restituisce una singola riga con gli eventi e i payload conservati, alla revisione richiesta. 404 se la riga manca in quel prefisso. Il browser mantiene una pagina di sintesi e un dettaglio alla volta; i payload non arrivano nello snapshot iniziale. La singola risposta del dettaglio non è suddivisa in chunk: resta soggetta ai limiti di cattura scelti.

`GET /api/v1/report?through=N&recordingId=ID`

Restituisce `version`, `revision`, `session`, tutte le righe sintetiche, metriche e timeline dell’intera registrazione. Non segue i filtri della vista e non restituisce i corpi dei payload. Il report HTML usa queste metriche senza ricomputarle nel browser, conserva intervalli osservati e copertura, escaping e CSP. Solo la richiesta esplicita di export carica tutte le sintesi nel browser.

## Misure temporali

- `durationMs`: misura disponibile della risposta, con la provenienza dell’evento. Zero esplicito è valido; una durata legacy assente resta sconosciuta. Chiusure sintetiche e rifiuti non inventano una durata.
- `toolDurationMs`: campione ammesso per percentili/classifiche: tool completato `ok|error`, durata nota non negativa e nessuna attesa storica di approvazione non separabile. Mediana e p95 usano nearest rank; `latencySamples=0` rende i valori non disponibili nella UI.
- `span.startMs/endMs`: timestamp catturati in millisecondi Unix. Fine presente solo per una coppia abbinata, non sintetica, con timestamp validi e non invertiti. Un intervallo a zero è valido. Le altre chiamate rimangono punti.
- `timeline.startMs/endMs/spanMs`: intervallo della registrazione fino all’ultima evidenza osservata; nello scope nativo, prima/ultima evidenza dello scope. Non cresce con l’orologio del browser.
- `coveredMs`: unione degli intervalli tool abbinati. `unattributedMs` è il complemento nell’intervallo osservato. Può comprendere modello, utente, scheduler e attività senza evidenze; non è una misura del ragionamento né una diagnostica di perdita di eventi.
- `overlapMs`: somma delle lunghezze degli intervalli meno l’unione; con tre o più chiamate parallele può superare il tempo trascorso con concorrenza. `peakConcurrency` usa solo intervalli positivi, con fini prima degli inizi coincidenti.
- `incompleteCalls`: chiamate senza una fine di cattura misurabile, comprese osservazioni senza risposta, ambiguità e chiusure sintetiche.
- `windowStartMs/windowEndMs`: finestra visualizzata della pagina corrente, separata dai totali dello scope. `gaps` contiene al massimo i primi 200 intervalli non attribuiti di questa finestra; `gapCount` dichiara il totale. La UI rende visibile l’eventuale limite. Nel report, senza finestra di pagina, i gap si riferiscono all’intera registrazione.

La waterfall mostra una riga per chiamata e le sovrapposizioni su un asse condiviso della pagina. Le durate native non vengono usate come larghezza delle barre di cattura. Non vengono ricostruiti turni o span LLM.

## Live e limiti

Il cockpit aggiorna la prima pagina live con richieste seriali ogni due secondi; pagine fissate da un cursore e registrazioni concluse non vengono ricaricate automaticamente. Il dettaglio usa la stessa revisione della lista. Refresh latest sblocca lo snapshot. Gli endpoint precedenti e `/ws` restano compatibili; il cockpit nuovo non scarica il loro snapshot completo.

Il server conserva una sola proiezione corrente e ricostruisce prefissi per cursori precedenti. Il recorder mantiene ancora tutti gli eventi in memoria e il sink rimane sincrono. Questa API limita il trasferimento ordinario verso il browser: non implementa code limitate, recupero del sink o memoria totale limitata. Il lettore JSONL ora recupera un frammento malformato solo a EOF, con qualità dichiarata; questo non implica recupero dei gap di persistenza o live. Questi lavori della fase 2 restano aperti.
