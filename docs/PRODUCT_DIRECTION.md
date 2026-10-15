# Direzione di prodotto

Stato: specifica di destinazione, 15 ottobre 2026. Punto di ingresso: [indice](README.md).

## Promessa

**The Local Profiler & Flight Recorder for AI Agents.**

Sentinel aiuta uno sviluppatore a ispezionare le azioni osservabili del proprio coding agent, trovare colli di bottiglia e ripetizioni e conservare una traccia utile per il debugging. La metafora di riferimento è il pannello Network di DevTools: richieste, risposte, tempi, errori e dettagli consultabili.

Il prodotto deve rispondere a una domanda concreta: **«Cosa posso cambiare nel prossimo tentativo, sulla base di quello che è successo in questo?»**

La leggerezza dell'installazione e la conservazione locale dei dati sono scelte di prodotto. La domanda di mercato e la disponibilità a pagare sono ipotesi da validare; non assumiamo un mercato vuoto o l'assenza di concorrenti.

## Utente e momento d'uso

Utente principale: uno sviluppatore che usa Cursor o Claude Code su un progetto locale e vuole capire una sessione lenta, ripetitiva o fallita. Apre Sentinel mentre l'agente lavora oppure subito dopo un problema.

Utente secondario: chi sviluppa un server MCP e vuole ispezionare il traffico stdio, correlare richieste e risposte e riprodurre un problema. Gli script custom sono coperti quando usano un trasporto già supportato; usare Ollama o vLLM, da solo, non rende un agente osservabile da Sentinel.

Reviewer e colleghi possono consultare una traccia esportata. Non sono il pubblico su cui modellare la prima schermata.

## Problemi prioritari

1. **Tempo difficile da attribuire.** Una sessione è durata molto: quali tool hanno occupato gli intervalli più lunghi? Quali parti non sono osservabili?
2. **Risposte sproporzionate.** Quali tool hanno prodotto più testo o dati? Ci sono letture o risultati ripetuti?
3. **Tentativi che non convergono.** Lo stesso comando fallisce più volte? Gli argomenti cambiano? Ci sono modifiche osservate tra un tentativo e il successivo?
4. **Evidenze disperse.** Quale richiesta ha causato quell'errore? Posso ritrovare payload e sequenza dopo aver chiuso l'agente?

## Prima esperienza utile

Lo sviluppatore collega un progetto con gli hook oppure configura un server MCP tramite proxy. Avvia la registrazione e continua a usare l'agente normalmente.

Nel cockpit trova:

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
- replay di tracce nuove e precedenti, report HTML autonomo e JSONL esportabile;
- visibilità sui limiti del recorder: troncamenti, correlazioni ambigue, dati persi.

Esclusi:

- firewall di comandi, valutazione del rischio, approvazioni e circuit breaker di spesa;
- tracing universale di qualsiasi agente o provider;
- accesso implicito alle API del modello, ai transcript o al filesystem del progetto;
- conteggio esatto del contesto, fatturazione reale o tempo interno di ragionamento;
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

La prima release è utile se, su una sessione reale problematica, uno sviluppatore riesce a individuare una chiamata lenta, un output sproporzionato o una sequenza di tentativi, aprirne le prove e decidere una modifica al tentativo successivo.

Validazione iniziale manuale: raccogliere sessioni con problemi noti e sessioni senza quei problemi; verificare utilità e falsi positivi delle segnalazioni. Annotare il risultato localmente senza introdurre telemetria di utilizzo inviata dal prodotto.

Conteggi di tool, token stimati e grafici da soli non dimostrano produttività o risparmio. Confronti tra agenti o prompt arrivano dopo, con task, configurazione e condizioni comparabili dichiarati.
