# Direzione di prodotto

Stato: direzione aggiornata su indicazione dell’utente il 16 ottobre 2026. Punto di ingresso: [indice](README.md). Sequenza di consegna: [piano di completamento](COMPLETION_PLAN.md). Le funzioni previste qui non sono tutte implementate; lo stato corrente è in [PROGRESS.md](PROGRESS.md).

## Promessa

**An analytics platform for monitoring and analyzing AI agent usage and metrics.**

Sentinel permette di monitorare utilizzo, prestazioni osservabili, errori, volumi e usage disponibile degli agenti AI nel tempo. Aggrega sessioni e progetti, evidenzia trend e anomalie e permette di confrontare run. Recorder, profiler e inspector raccolgono i dati e consentono di approfondire ogni misura fino alle evidenze.

Il prodotto deve rispondere a domande concrete: **«Come sto usando i miei agenti? Cosa sta cambiando? Dove si concentrano tempo, errori e usage? Quali differenze emergono tra due run?»**

La leggerezza dell'installazione e la conservazione locale dei dati sono scelte di prodotto. La domanda di mercato e la disponibilità a pagare sono ipotesi da validare; non assumiamo un mercato vuoto o l'assenza di concorrenti.

## Utente e momento d'uso

Utente principale: uno sviluppatore che usa agenti AI su uno o più progetti e vuole seguirne utilizzo e metriche sia durante il lavoro sia su giorni e settimane. Cursor e Claude Code sono le integrazioni iniziali, non il limite concettuale della piattaforma.

Utente secondario: chi sviluppa un server MCP e vuole ispezionare il traffico stdio, correlare richieste e risposte e riprodurre un problema. Gli script custom sono coperti quando usano un trasporto già supportato; usare Ollama o vLLM, da solo, non rende un agente osservabile da Sentinel.

Reviewer e colleghi possono consultare report esportati. Il monitoraggio parte locale; un prodotto cloud o di team non è implicito nella parola piattaforma.

## Problemi prioritari

0. **Utilizzo frammentato tra sessioni.** Quanta attività è stata osservata, su quali progetti e agenti, e come varia rispetto al periodo precedente? Quale copertura hanno i dati?
1. **Tempo difficile da attribuire.** Una sessione è durata molto: quali tool hanno occupato gli intervalli più lunghi? Quali parti non sono osservabili?
2. **Risposte sproporzionate.** Quali tool hanno prodotto più testo o dati? Ci sono letture o risultati ripetuti?
3. **Tentativi che non convergono.** Lo stesso comando fallisce più volte? Gli argomenti cambiano? Ci sono modifiche osservate tra un tentativo e il successivo?
4. **Evidenze disperse.** Quale richiesta ha causato quell'errore? Posso ritrovare payload e sequenza dopo aver chiuso l'agente?

## Prima esperienza utile

Lo sviluppatore collega un progetto con gli hook oppure configura un server MCP tramite proxy. Avvia la registrazione e continua a usare l'agente normalmente.

La home è la Timeline per analizzare le chiamate: una riga leggibile per ogni tool, argomenti a fianco dell’intervallo catturato, durata disponibile e selezione collegata all’inspector. Le statistiche generali sono nella vista separata Statistics, con periodo, progetto e agente selezionabili, trend compatti e accesso alle sessioni sottostanti. Questa gerarchia riflette la correzione esplicita dell’utente del 16 ottobre 2026.

La navigazione attuale comprende Timeline, Sessions, Statistics, Tool profile ed Event log. Diagnostica, Usage e Compare estenderanno il percorso di analisi. Le sezioni Usage e Compare dichiarano dati disponibili e copertura; i dettagli dell’interfaccia verranno progettati durante l’implementazione.

Dentro una sessione trova:

- la sessione selezionata e la copertura dei dati disponibili;
- una waterfall di chiamate, con sovrapposizioni e intervalli non attribuiti;
- un riepilogo di chiamate, errori, durate e byte osservati;
- le risposte più grandi e le sequenze ripetute, ordinate e apribili;
- un dettaglio con argomenti, risultato, origine dei tempi ed eventuali limiti di cattura;
- replay locale ed esportazione di una traccia o di un report consultabile offline.

Esempio **illustrativo**, non risultato misurato del progetto: «Lo stesso comando è fallito quattro volte in tre minuti. Nessuna modifica di file è stata osservata tra questi tentativi». Il collegamento apre le quattro coppie richiesta/risposta e segnala se la cattura delle modifiche era parziale.

La vista iniziale non è un muro di JSON né un punteggio astratto dell'agente. Deve permettere di passare da una sintesi a un evento verificabile.

## Confini della prima release

Inclusi:

- hook Cursor e Claude Code e proxy MCP stdio già presenti, adattati all'osservazione passiva;
- separazione delle sessioni dell'agente dentro la registrazione del processo Sentinel;
- waterfall dei tool, metriche con denominatori espliciti, ispezione dei payload;
- segnalazioni deterministiche di ripetizioni, errori ricorrenti e output grandi;
- catalogo persistente multi-sessione con filtri per periodo, progetto e agente quando noti;
- dashboard aggregata, trend giornalieri e breakdown per agente, progetto e tool;
- usage nativo attraverso almeno un importatore esplicito, con contatori, modello e provenienza quando disponibili;
- confronto A/B di sessioni selezionate con condizioni, copertura e differenze osservate dichiarate;
- replay di tracce nuove e precedenti, report HTML autonomo e JSONL esportabile;
- visibilità sui limiti del recorder: troncamenti, correlazioni ambigue, dati persi.

Esclusi:

- firewall di comandi, valutazione del rischio, approvazioni e circuit breaker di spesa;
- tracing universale di qualsiasi agente o provider;
- accesso implicito alle API del modello, ai transcript o al filesystem del progetto;
- ricostruzione del contesto, fatturazione o ragionamento interno dai soli hook; eventuali costi nativi riportati e scenari da listino restano distinti;
- valutazione automatica della qualità del codice o del successo del task dai soli tool;
- esecuzione automatica di rimedi, rilancio di comandi o riproduzione delle azioni;
- piattaforma cloud, collaborazione live, database esterni obbligatori e redesign della landing page.

## Come trattare la sicurezza esistente

Il nuovo percorso di registrazione non valuta le vecchie policy e non richiede conferme. Il codice precedente può essere disaccoppiato per passi, ma non diventa una seconda area di prodotto da sviluppare insieme al profiler.

Le tracce storiche possono contenere blocchi, attese e rifiuti: restano visibili come eventi storici. Non vanno reinterpretati come errori del tool né rimossi dalle evidenze originali.

La migrazione da installazioni con protezioni attive deve essere esplicita: nessun aggiornamento silenzioso degli hook o delle aspettative dell'utente. Modalità e tempi sono definiti nella [roadmap](ROADMAP.md).

La protezione dei dati raccolti resta necessaria: minimizzazione, redazione configurabile, accesso locale e controllo dell'export servono al recorder, senza trasformarlo in un filtro dei comandi.

## Linguaggio delle misure

- «Durata osservata della chiamata», non «tempo di pensiero».
- «Byte della risposta osservata» e «token stimati del testo osservato», non «contesto consumato».
- «Possibile sequenza di retry», non «agente impazzito».
- «Nessuna modifica osservata», non «nessun progresso».
- Dato assente: «non disponibile», mai zero implicito.

«Offline» significa che Sentinel non richiede invii remoti per registrare, analizzare o rivedere una traccia. L'agente e i server MCP possono usare la rete per conto proprio. «Setup breve» descrive meglio l'obiettivo rispetto a «zero-config»: hook e proxy richiedono comunque un collegamento iniziale.

## Segnali di riuscita

Il primo MVP analytics è utile se uno sviluppatore ritrova le sessioni dopo un riavvio, filtra un periodo/progetto/agente, individua una variazione delle metriche e apre le sessioni che la spiegano. La release analytics completa aggiunge diagnostica, usage nativo da almeno una sorgente e confronto A/B, con report riproducibili.

Validazione iniziale manuale: raccogliere sessioni con problemi noti e sessioni senza quei problemi; verificare utilità e falsi positivi delle segnalazioni. Annotare il risultato localmente senza introdurre telemetria di utilizzo inviata dal prodotto.

Conteggi di tool, token stimati e grafici da soli non dimostrano produttività o risparmio. I confronti fanno parte della piattaforma e dichiarano task, configurazione, condizioni iniziali e copertura; una differenza osservata non diventa automaticamente una classifica di qualità.
