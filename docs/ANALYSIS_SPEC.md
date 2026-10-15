# Analisi delle sessioni

Stato: specifica di destinazione. Le regole seguenti sono deterministiche e locali; non sono già implementate. Dati e qualità seguono il [contratto di telemetria](TELEMETRY_SPEC.md).

## 1. Contratto dei risultati

Una metrica descrive il dato; un finding segnala una sequenza da ispezionare. Nessuno dei due dimostra da solo una causa, un errore di ragionamento o uno spreco economico.

Ogni finding contiene:

- `id` stabile per input e versione identici;
- `ruleId`, versione della regola e parametri effettivi;
- sessione e intervallo temporale osservato;
- titolo concreto, spiegazione e priorità diagnostica;
- riferimenti `(recordingId, seq)` a tutte le evidenze necessarie;
- misure con unità, denominatori e copertura;
- qualità dei dati e condizioni che limitano l'interpretazione;
- una possibile verifica manuale, senza azioni eseguite automaticamente.

Non usare percentuali di confidenza inventate. Distinguere fatto misurato, somiglianza euristica e dato insufficiente. Se manca il prerequisito per una regola, segnalarla come non disponibile anziché produrre un verdetto.

Gli ID e il formato di persistenza dei findings vanno fissati con fixture. Una ricomputazione non modifica gli eventi sorgente. Un report deve dichiarare la versione dell'analisi usata.

## 2. Metriche di base

### Chiamate ed esiti

Contare richieste osservate e chiamate parziali separatamente. Per ogni tool/server mostrare successi riportati, errori riportati, interruzioni, chiamate aperte ed esiti sconosciuti.

Il tasso di errore è `error / (ok + error)`, con numeratore, denominatore e intervallo visibili. Interruzioni, incomplete e decisioni storiche di policy sono conteggi separati. Se il denominatore è zero il tasso è non disponibile. Non chiamarlo affidabilità assoluta del server.

### Tempi

Ordinare durate valide e mostrare conteggio, mediana e p95 con algoritmo dichiarato. Usare nearest rank: per `n > 0`, indice a base zero `max(0, ceil(q * n) - 1)`. Nessun campione produce un valore non disponibile; zero millisecondi è un campione valido.

Non mescolare durate misurate nel proxy, riportate dagli hook e ricostruite senza renderne visibile la provenienza. Escludere attese di approvazione storiche non separabili, durate sconosciute, rifiuti e interruzioni. Mostrare quanti campioni sono stati esclusi e perché.

La somma delle durate è «tempo cumulato delle chiamate» e può superare il tempo trascorso se ci sono tool paralleli. Il tempo coperto nella waterfall usa l'unione degli intervalli validi, non la somma.

### Volumi

Mostrare byte osservati, byte conservati, testo disponibile e risposte più grandi. Aggregare per sessione, tool e server quando noto. Presentare copertura e quote sullo stesso insieme di dati; una quota di payload non è una quota del contesto del modello.

Token testuali eventualmente stimati sono una vista secondaria con metodo dichiarato. Nessuna stima di dollari o di context occupancy nella prima release.

## 3. Regole della prima release

Le soglie seguenti sono **default iniziali proposti**, non valori validati su utenti. Devono essere parametri versionati; modificarli non cambia retroattivamente l'evidenza originale.

### R1 — Risposta grande

- Prerequisito: dimensione del testo osservato prima del troncamento, oppure indicazione esplicita che la misura è parziale.
- Trigger iniziale: almeno 64 KiB di testo osservato in una singola risposta.
- Evidenza: chiamata, risposta, dimensione e quota del testo osservato nella sessione; su un turno solo se quel confine è disponibile.
- Output: «Questa risposta contiene 96 KiB di testo osservato», con unità e limiti. Il numero è illustrativo.
- Non affermare che il testo sia stato letto dal modello o sia inutile. I payload binari sono classificati separatamente.

Anche senza soglia superata, la graduatoria delle risposte più grandi resta disponibile. Una sorgente con testo omesso non deve apparire come un tool economico da zero byte.

Se si conosce solo il prefisso conservato e questo supera la soglia, il finding indica «almeno» quella dimensione. Non attribuire una quota esatta sul totale quando le dimensioni mancanti rendono il denominatore incompleto.

### R2 — Chiamata identica ripetuta

- Ambito: stessa sessione dell'agente e stessa sorgente/server; mai conversazioni diverse.
- Firma: nome tool e argomenti JSON canonici con chiavi ordinate ricorsivamente. Conservare valori, array, spazi nelle stringhe e tipi; non normalizzare comandi shell in modi che ne cambino il significato.
- Trigger iniziale: almeno quattro chiamate completate con la stessa firma in una finestra mobile di cinque minuti.
- Output: «Quattro chiamate con gli stessi argomenti», con esiti e tempi. Non chiamarle automaticamente retry falliti.
- Controesempio da gestire: polling legittimo. Presentare il fatto come informazione; raggruppare polling riconosciuto solo con configurazione esplicita o evidenze disponibili.

La somiglianza tra argomenti diversi è rinviata: richiede una regola distinta e non deve indebolire la precisione della firma esatta.

Con argomenti troncati o redatti, l'uguaglianza del contenuto conservato non prova l'uguaglianza delle chiamate originali: R2 e R3 non sono applicabili per quella firma. Non conservare hash di segreti o copie nascoste del payload per aggirare la redazione. Per sorgenti senza sessione nativa, il collegamento MCP isolato può essere l'ambito dichiarato; eventi hook di conversazioni non distinguibili non vanno aggregati come una sessione certa.

### R3 — Stesso errore in tentativi ripetuti

- Prerequisito: R2 e almeno tre esiti di errore osservati nel gruppo.
- Confrontare codice e categoria dell'errore; per testo usare una firma separata e dichiarata. Non eliminare percorsi, valori o numeri arbitrariamente per forzare equivalenze.
- Evidenza: tentativi falliti, eventuali successi intermedi, argomenti, errore disponibile e modifiche file osservate tra i tentativi.
- Output: «Tre tentativi con gli stessi argomenti hanno riportato lo stesso errore» solo se la firma dell'errore coincide. Altrimenti «Tre tentativi sono falliti», con errori distinti consultabili.
- «Nessuna modifica osservata» è utilizzabile solo accompagnata dalla copertura delle modifiche. Non significa che il repository o l'ambiente non siano cambiati.

Un finding R3 incorpora R2 nel riepilogo per non generare due avvisi equivalenti sulla stessa sequenza. Le evidenze restano consultabili e il conteggio delle chiamate non viene duplicato.

### R4 — Chiamata lunga

- Trigger iniziale: durata valida di almeno dieci secondi.
- Output: durata, provenienza del tempo, tool/server, risultato e posizione nella waterfall.
- Non attribuire la causa al server: una build o un test può impiegare legittimamente quel tempo. Non chiamare «timeout» una chiamata solo perché lunga.
- Una chiamata aperta può avere un indicatore live di tempo trascorso, ma non entra nei percentili delle chiamate concluse.

Il confronto con la latenza tipica dello stesso tool può arrivare dopo, con un numero minimo di campioni dichiarato e senza confrontare argomenti eterogenei come se fossero lo stesso workload.

## 4. Analisi successive

### Cicli di modifiche e test

Prima versione successiva: segnalare una sequenza osservata edit → test fallito → edit → stesso test fallito. Per parlare di «annullamento» o ping-pong servono patch o stati confrontabili che dimostrino il ritorno al contenuto precedente. Il solo nome del file non basta.

Non leggere retroattivamente il workspace per inventare gli stati mancanti. Non interpretare l'esito di un test da testo ambiguo quando manca un risultato affidabile.

### Diagnostica MCP e validazione degli argomenti

Prerequisiti: catalogo `tools/list` effettivamente osservato, relativo server/connessione e schema valido con dialetto supportato. Associare ogni chiamata allo schema noto in quel momento; gestire paginazione e cambi del catalogo prima di dichiararlo completo.

Distinguere errore JSON-RPC, risultato MCP con `isError`, guasto del trasporto e argomento che non soddisfa lo schema. Un errore del tool non dimostra un'allucinazione del modello.

Validazione diagnostica, senza bloccare o modificare i frame. Se schema o dialetto non sono disponibili/supportati, mostrare «non validato». Non chiamare questa funzione certificazione di conformità MCP.

### Confronto tra sessioni

Richiede selezione esplicita di due sessioni e metadati comparabili: task, agente, modello quando noto, configurazione e stato iniziale dichiarato. Mostrare differenze osservate, senza costruire classifiche di qualità o causalità da condizioni diverse.

### Transcript e usage nativo

Importatori opzionali potranno aggiungere turni, messaggi e usage. Servono contratti separati per provenienza, consenso alla lettura, redazione, conteggi cumulativi e deduplicazione. Un transcript non espone automaticamente il ragionamento interno del modello.

## 5. Presentazione e priorità

Ordine iniziale: errori ripetuti, risposte grandi, chiamate lunghe, ripetizioni senza errore. La priorità serve alla navigazione e non rappresenta un rischio di sicurezza.

Ogni scheda apre la selezione corrispondente nella timeline. Il dettaglio mostra regola e parametri, dati usati e dati mancanti. Un badge di cattura incompleta deve essere vicino alla diagnosi, non nascosto in una pagina secondaria.

Assenza di findings significa «nessuna regola applicabile ha superato le soglie sui dati disponibili», non «sessione efficiente» o «task riuscito». Il replay ricostruisce gli eventi; non rilancia tool o comandi.

## 6. Verifica delle analisi

Fixture richieste: chiamata lenta isolata, output grande Unicode, output omesso, quattro retry con errore, polling valido, chiamate uguali concorrenti, due conversazioni con stesso comando, gap di cattura e sessione senza anomalie.

Le asserzioni devono coprire risultati, evidenze, denominatori e soppressioni. Includere finestre al limite della soglia, campioni a zero millisecondi e chiamate aperte. L'analisi dello stesso snapshot deve coincidere tra live, replay ed export.

Una fixture sintetica prova il comportamento della regola; non dimostra utilità per gli utenti. La revisione su sessioni reali è un criterio separato della roadmap.
