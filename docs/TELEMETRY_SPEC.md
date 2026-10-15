# Contratto di telemetria

Stato: contratto di destinazione con un sottoinsieme implementato dichiarato sotto. Le sezioni successive comprendono anche lavoro ancora previsto. Vedi [stato](PROGRESS.md), [architettura](ARCHITECTURE.md) e [roadmap](ROADMAP.md).

## 0. Contratto v2 attualmente implementato

La nuova scrittura è un’estensione additiva dell’envelope precedente. Identità, correlazione e profili di cattura sono implementati; non introduce ancora l’envelope `data`, capability o paginazione descritti più avanti.

- `schemaVersion: 2`, `recordingId`, `seq`; `sessionId` mantiene il significato storico di registrazione Sentinel ed è uguale a `recordingId`.
- `timestamp` rimane il nome serializzato dell’istante UTC di osservazione locale. `observedAt`, timestamp nativi e turni non sono ancora normalizzati.
- `source` è `cursor-hook`, `claude-hook` o `mcp-stdio`; `sourceId` distingue l’adattatore nel collector corrente o la connessione del proxy. Non è un’identità verificata dell’installazione dell’IDE o dell’host.
- `agentSessionId` è il valore nativo quando trasmesso. `connectionId` è disponibile per il proxy stdio; gli hook HTTP non inventano una connessione persistente. `projectRef` conserva il cwd ricevuto quando disponibile, senza leggere il progetto.
- `callId` identifica una chiamata osservata, compresa un’osservazione parziale. `nativeCallId` è un oggetto `{ "type": "string|number|null", "value": "valore esatto" }`: anche i numeri sono memorizzati come testo per evitare arrotondamenti nel browser.
- `correlation` espone `method`, `quality`, `requestSeq` quando la risposta è abbinata, `candidateCallIds` per le sovrapposizioni e `synthetic` per le chiusure ricostruite. Gli eventi `CaptureDiagnostic` dichiarano le ambiguità senza mutare gli eventi precedenti.
- `timing` contiene `durationMs`, incluso zero, e `source: source-reported|proxy-monotonic`. Il vecchio campo `durationMs` resta un alias non autorevole per compatibilità. Senza `timing`, una durata legacy assente resta sconosciuta.
- I campi esistenti `type`, `direction`, `method`, `toolName`, `rpcId`, `status` e `payload` restano leggibili. `rpcId` negli hook è un alias sintetico storico, distinto dall’ID nativo.
- `SessionStarted.captureSettings` e `Session.capture` conservano `{profile, maxPayloadBytes, redaction}`. La stessa configurazione è disponibile in `/api/config.capture`, anche nel replay. Un log precedente senza configurazione resta senza configurazione dichiarata.
- Ogni payload nuovo ha `capture`: `representation` (`hook-normalized`, `mcp-frame`, `text` o `unspecified`), `completeness` (`complete`, `partial`, `omitted`, `unknown`), `profile`, `normalizedBytes` e `storedBytes`.
- Quando misurabile, `observedBytes` ha `observedUnit` (`utf8-text`, `json-value`, `mcp-frame`) e `observedField` per gli hook. Misura il valore selezionato dall’adattatore prima della normalizzazione: non l’intero body HTTP o il contesto del modello. Nel proxy misura il frame prima dei delimitatori di riga. Uno zero esplicito resta valido.
- `sourceReportedBytes` e `sourceReportedUnit` descrivono una misura trasmessa dal client hook, distinta dai byte arrivati al collector. Se il client ha già accorciato il contenuto, il profilo full non lo ricostruisce.
- `normalizedBytes` e `storedBytes` usano il valore JSON del payload, escluso l’envelope dell’evento. `changes` dichiara `kind` (`normalized`, `redacted`, `truncated`, `omitted`), `stage` (`hook-client`, `adapter`, `recorder`), `method`, conteggio e byte prima/dopo quando disponibili.
- Il profilo standard redige i campi strutturati e i pattern testuali riconosciuti prima di memoria, sink e subscriber live. Le stringhe di identità che corrispondono ai pattern sensibili diventano HMAC opachi con chiave casuale per registrazione, mantenendo distinte le identità; `metadataRedactedFields` dichiara i campi modificati. La chiave non viene persistita. Gli ID numerici conservano la precisione.

### Profili e limiti implementati

`serve` e `mcp` accettano `--capture standard|full` e `--max-payload-bytes N`. Standard è predefinito: limite 65.536 byte e redazione `structured-text-v1`; full è esplicito: limite 1.048.576 byte e `redaction: none`. Un limite esplicito deve essere tra 256 e 16.777.216 byte; zero sceglie il default del profilo. Sono limiti per singolo payload conservato, non della memoria totale o dell’input ricevuto.

La redazione è best effort su nomi sensibili e pattern riconoscibili, non una garanzia di rilevare tutti i segreti. Avviene prima del limite `json-prefix-v1`: il valore conservato resta JSON valido e UTF-8, con stringhe abbreviate e possibili elementi/proprietà omessi in ordine deterministico. La qualità dichiara questa perdita; non viene inserito un campo fittizio nel risultato del tool. L’inoltro MCP rimane invariato.

Standard omette il contenuto delle letture Cursor e sintetizza le risposte Read/Edit di Claude; full conserva il contenuto ricevuto e l’intero valore `tool_response` dei PostToolUse. Le richieste e alcuni risultati degli hook restano normalizzati: quando non è garantita la conservazione del valore osservato la copertura è `unknown`. `complete` si riferisce solo al valore e all’unità dichiarati.

Il cockpit mostra qualità e misure per richiesta/risposta, anche accanto alle patch. Le classifiche continuano a usare il JSON conservato, senza confrontarlo implicitamente con testo o frame originali. Il report HTML conserva profilo e sintesi della copertura, con escaping e CSP. Il replay non redige nuovamente o riscrive log precedenti; dimensioni originali e qualità assenti restano sconosciute.

Una richiesta con identità disponibile può iniziare con qualità `exact` e rimanere in attesa della risposta. Se compare una sovrapposizione, la diagnostica marca l’intero gruppo come ambiguo nella proiezione. `exact` indica un abbinamento univoco rispetto alle evidenze disponibili; con `method: fallback` rimane un’inferenza, non un ID nativo.

Il fallback usa sessione nota, tool/server e argomenti canonici originali, prima di eventuali riduzioni nella normalizzazione. Senza sessione non viene tentato un abbinamento euristico. Il gruppo ambiguo resta tale finché tutte le risposte attese sono state osservate; non si assegna la seconda risposta rimasta a una richiesta scelta arbitrariamente. Una risposta orfana non genera una richiesta sintetica. Le chiusure per Stop/SessionEnd sono marcate come sintetiche e non forniscono una durata di esecuzione.

Il lettore conserva i vecchi envelope senza migrazioni su disco, mantiene tipi di evento non interpretati con il loro payload e rifiuta major non supportate, identità miste o sequenze v2 non crescenti. Il recupero della coda incompleta è ancora previsto. Fixture eseguibile nel replay: [`../testdata/telemetry-v2.jsonl`](../testdata/telemetry-v2.jsonl).

Contratti esterni consultati il 15 ottobre 2026: [Cursor hooks](https://cursor.com/docs/hooks), [Claude Code hooks](https://code.claude.com/docs/en/hooks), [JSON-RPC 2.0](https://www.jsonrpc.org/specification). L’adattatore Claude accetta il campo opzionale `duration_ms` già previsto dal codice; non presume che ogni versione o evento lo trasmetta. Non è stata eseguita una verifica live delle applicazioni esterne.

## 1. Principio

Ogni misura deve dichiarare **cosa è stato osservato, da quale sorgente e con quali limiti**. Le lacune di cattura fanno parte del dato. Distinguere sempre: dato misurato, dato riportato dalla sorgente, stima e dato non disponibile.

## 2. Identità e gerarchia

- `recordingId`: registrazione del collector o del proxy Sentinel. Non coincide necessariamente con un task.
- `agentSessionId`: sessione o conversazione nativa, accompagnata da tipo di agente e namespace dell'istanza. Assente se non ricavabile.
- `connectionId`: istanza di collegamento a una sorgente; isola riavvii e server con ID RPC uguali.
- `turnId`: turno dichiarato dalla sorgente. Non derivarlo automaticamente dal silenzio tra tool.
- `callId`: identità interna univoca della chiamata, stabile nel replay.
- `nativeCallId`: ID originale del tool o RPC con tipo conservato. La stringa `"1"` e il numero `1` non sono lo stesso ID.
- `seq`: sequenza strettamente crescente dentro una registrazione. Il riferimento completo a un evento è `(recordingId, seq)`.
- `projectRef`: riferimento al progetto, solo quando comunicato dalla sorgente o scelto esplicitamente. La normalizzazione del percorso non deve leggere il contenuto del progetto.

Nella prima release l'attuale `SessionID` dei log precedenti mappa a `recordingId`. Non prova che tutti gli eventi appartengano a una sola conversazione dell'agente.

## 3. Correlazione delle chiamate

Usare prima l'identità nativa con il namespace completo di sorgente, connessione e sessione. I frame nelle due direzioni possono avere ID coincidenti: includere il lato che ha originato la richiesta.

Quando l'hook non ha un ID nativo, usare una correlazione di ripiego dichiarata, basata su sessione, tool e argomenti canonici. Non fondere chiamate uguali concorrenti: se non sono distinguibili, segnare `ambiguous` e non attribuire una durata precisa a una coppia arbitraria.

Metadati minimi di correlazione:

- `method`: `native`, `fallback` o `none`;
- `quality`: `exact`, `ambiguous` o `unmatched`;
- riferimenti alla richiesta e alla risposta, se esistono;
- `synthetic`: se l'adattatore ha ricostruito un evento mancante.

Risposta senza richiesta: conservarla come tale. La UI può presentare una chiamata parziale, ma non inventare un timestamp di inizio. Eventi duplicati non devono chiudere due chiamate diverse; deduplicare solo con identità affidabile e conservarne il conteggio diagnostico.

## 4. Envelope degli eventi

Schema di nuova scrittura: `schemaVersion: 2`. Campi di base proposti:

- identità di registrazione, sequenza, tipo e sorgente;
- `observedAt`: UTC di ricezione o osservazione locale;
- `sourceTimestamp`: solo se trasmesso dalla sorgente, con provenienza;
- identità di sessione, connessione, turno e chiamata dove note;
- `status`, riferimenti a payload, limiti della cattura;
- `data`: campi specifici del tipo di evento.

Tipi concettuali richiesti:

- inizio/fine registrazione e sessione, quando il relativo confine è noto;
- inizio/fine turno, solo se esposto dalla sorgente;
- richiesta/risposta tool e richiesta/risposta/notifica di protocollo;
- modifica file quando riportata dall'agente;
- diagnostica della cattura, inclusa la perdita di eventi;
- dichiarazione o modifica delle capability della sorgente.

La prima implementazione deve fissare i nomi serializzati e le fixture insieme. Eventi futuri sconosciuti non vanno eliminati o resi fatali se l'envelope resta leggibile: mostrarli come non interpretati. Una major di schema incompatibile richiede invece un errore esplicito, senza presentare un'analisi parziale come completa.

## 5. Chiamata derivata

La proiezione richiesta/risposta deve esporre:

- identità e riferimenti alle evidenze;
- tool, server quando noto, argomenti e risultato disponibili;
- inizio/fine osservati, durata e origine della durata;
- dimensioni disponibili della richiesta e della risposta;
- esito e categoria dell'errore;
- qualità di correlazione, completezza e redazioni.

Esiti della nuova proiezione: `pending`, `ok`, `error`, `interrupted`, `unknown`. Gli esiti storici `blocked`, `rejected` e `awaiting_approval` restano leggibili separatamente.

Non interpretare output non vuoto su stderr come fallimento. `error` richiede un segnale disponibile: exit code non zero, errore JSON-RPC, `isError`, hook di fallimento o equivalente dichiarato. Se una sorgente non comunica l'exit code, dichiarare che l'esito non è completamente verificabile.

## 6. Tempi e waterfall

Ordine di preferenza della durata:

1. Misura locale con clock monotono, quando richiesta e risposta attraversano lo stesso processo e la correlazione è esatta.
2. Durata riportata dalla sorgente, con unità verificate dall'adattatore e provenienza esplicita.
3. Differenza tra timestamp di osservazione, etichettata come intervallo osservato e comprensiva degli eventuali ritardi degli hook.

Conservare separatamente le misure se più di una è disponibile. Il clock monotono non si serializza come valore assoluto: salvare la durata risultante. Timestamp incompatibili, mancanti o invertiti rendono la durata non disponibile e generano diagnostica. Una durata misurata di zero millisecondi è valida e diversa da un campo assente.

La waterfall usa l'origine del tempo scelta per ogni span e rende visibili sovrapposizioni e chiamate incomplete. Non sommare durate parallele per ottenere il tempo totale della sessione.

L'unione degli intervalli coperti dai tool è «tempo con tool osservati in corso». Il complemento dentro un intervallo di registrazione noto è «tempo non attribuito ai tool osservati». Può includere LLM, utente, scheduler, rete e lacune della cattura: non chiamarlo tempo di ragionamento.

Senza confini di turno affidabili mostrare una waterfall di sessione; non presentare turni inventati. In live il punto finale usato per i calcoli deve essere esplicito nello snapshot, così il replay non dipende dall'orologio del momento.

## 7. Dimensioni, testo e token

Conservare separatamente, quando disponibili:

- `observedBytes`: byte UTF-8 del payload arrivato al punto di cattura, prima delle trasformazioni di Sentinel;
- `storedBytes`: byte effettivamente conservati dopo redazione/troncamento;
- `textBytes`: byte del testo estratto con successo, distinto da JSON envelope, immagini e dati binari;
- `sourceReportedBytes`: dimensione riportata dalla sorgente, se non è una misura locale;
- `estimatedTextTokens`: stima sul testo disponibile, con metodo e versione;
- `reportedUsage`: usage nativo, solo in future integrazioni che ne dichiarano unità, modello, ambito e provenienza.

Non ricavare il volume originale dalla lunghezza di una stringa già troncata. Se l'hook è ridotto nel client, la misura prima della riduzione va trasmessa esplicitamente; se manca, la dimensione originale è sconosciuta. Non chiamare «raw» un output ricostruito o privato di parti.

La prima release può usare una semplice euristica testuale, purché etichettata e versionata. Deve stimare il testo osservato e dichiarare la copertura; non contare JSON, immagini o placeholder di redazione come token del modello. Se il testo è parziale, la stima è parziale.

Nessun contatore «percentuale della context window» senza dati nativi sull'effettivo input. La quota di un output può essere mostrata solo rispetto al **testo osservato** della stessa sessione/turno e con quel denominatore esplicito.

Il riepilogo della prima release non mostra dollari stimati dai vecchi prezzi fissi. Un futuro costo calcolato richiederà usage appropriato, modello, prezzi datati, cache e categorie di token pertinenti; non verrà chiamato fattura.

## 8. Capability per sorgente

Ogni sorgente dichiara disponibilità e copertura di: ID di sessione/turno/tool, coppie richiesta/risposta, esiti, durate, argomenti, contenuti, modifiche file, catalogo degli schemi MCP e usage nativo.

Base nota dal codice attuale:

- **Cursor hook:** eventi shell, letture e modifiche; eventi MCP solo se configurati. Non tutte le operazioni hanno una coppia completa e alcune risposte/contenuti sono ridotti. Nessun usage LLM nativo normalizzato.
- **Claude Code hook:** ID di sessione e tool use, pre/post/fallimento e segnali di chiusura. Non implica che ogni hook trasmetta tutti i contenuti o i tempi. Nessun usage LLM nativo normalizzato.
- **Proxy MCP stdio:** frame osservati sul collegamento configurato, con richieste, risposte e notifiche. Il catalogo dei tool è disponibile solo se visto transitare. Non vede automaticamente transcript, altri server, file locali o chiamate al provider.
- **JSONL precedente:** envelope originale, con metadati nuovi recuperabili solo in parte. Provenienza e limiti non presenti restano sconosciuti.

Una capability descrive sia ciò che l'adattatore sa leggere sia ciò che questa specifica registrazione contiene. I contratti esterni vanno verificati durante l'implementazione; aggiungere un nome di agente non basta a dichiarare compatibilità.

## 9. Cattura, conservazione e dati locali

I due profili implementati sono descritti nella sezione 0: `standard`, con limiti e redazione dei campi sensibili riconoscibili; `full`, scelto esplicitamente per conservare payload più completi senza redazione. Anche full dichiara limiti per payload ed eventuali omissioni a monte: non garantisce una cattura illimitata.

Redazione e limiti avvengono prima della persistenza e della trasmissione alla UI. I byte del traffico inoltrato restano intatti. Ogni trasformazione registra motivo, quantità quando nota e metodo; non promettere di rilevare tutti i segreti. Evitare di conservare copie raw parallele che aggirino il profilo scelto.

Le soglie di payload sono già esplicite e persistite. Soglie di coda e conservazione totale restano da implementare, insieme al dettaglio paginato. Quest’ultimo deve mostrare tutto il contenuto **conservato** senza troncarlo silenziosamente a schermo.

La cattura non legge file solo per completare un diff o indagare il repository. Usa quanto ricevuto; futuri snapshot del workspace richiedono una funzione separata e consapevole.

## 10. Qualità della registrazione

Distinguere almeno:

- eventi scartati prima della persistenza;
- eventi mancati solo dal subscriber live e recuperabili;
- errori del sink e intervalli non persistiti;
- payload omessi, troncati o redatti;
- correlazioni ambigue e confini di sessione/turno assenti.

I gap noti devono essere marcati anche nei risultati analitici coinvolti. Se il disco è indisponibile, esporre il problema in memoria e su stderr; al ripristino registrare una diagnostica del gap. Dopo un crash non è garantita la ricostruzione dell'intervallo perso: indicare chiusura incompleta, senza inventarne la dimensione.

## 11. Compatibilità e recupero

Leggere le tracce esistenti senza modificarle. Conservare i campi e i verdict storici; non rieseguire policy o comandi durante il replay.

Un file interrotto può avere un'ultima riga JSON incompleta: il lettore nuovo può recuperare il prefisso valido dichiarando la coda incompleta. Corruzione nel mezzo richiede una diagnostica con posizione e una scelta esplicita di recupero; non saltare righe in silenzio.

Versionare separatamente schema dei dati e analisi. Le fixture di compatibilità includono log precedenti, tracce incomplete, eventi futuri, più sessioni, concorrenza e payload oltre limite.
