# Contratto di telemetria

Stato: proposta implementativa per la nuova release. Nessuno dei nuovi campi descritti qui è da presumere già disponibile nel codice. Vedi [architettura](ARCHITECTURE.md) e [roadmap](ROADMAP.md).

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

Prevedere due profili: `standard`, con limiti e redazione dei campi sensibili riconoscibili; `full`, scelto esplicitamente per conservare payload più completi. Anche il profilo full dichiara limiti di risorse ed eventuali omissioni a monte: non garantisce una cattura illimitata.

Redazione e limiti avvengono prima della persistenza e della trasmissione alla UI. I byte del traffico inoltrato restano intatti. Ogni trasformazione registra motivo, quantità quando nota e metodo; non promettere di rilevare tutti i segreti. Evitare di conservare copie raw parallele che aggirino il profilo scelto.

Le soglie di payload, coda e conservazione devono essere esplicite nella configurazione e riportate nella traccia. Il dettaglio paginato può mostrare tutto il contenuto **conservato** senza troncarlo silenziosamente a schermo.

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
