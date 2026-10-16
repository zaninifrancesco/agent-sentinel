# Roadmap del pivot

Direzione aggiornata il 16 ottobre 2026: **an analytics platform for monitoring and analyzing AI agent usage and metrics**. Il [piano di completamento](COMPLETION_PLAN.md) definisce ora l’ordine di consegna: fondazione multi-sessione → dashboard e trend → diagnostica → usage nativo → confronto e report → chiusura della release. Le fasi tecniche sotto restano il riferimento per dipendenze e accettazione; la loro precedente definizione di prima release è superata dal perimetro analytics. Catalogo e dashboard non attendono il completamento di ogni attività di hardening.

Stato: la fase 1 è implementata sul branch `telemetry-profiler`; la fase 2 è in corso, con identità, correlazione, selezione delle sessioni, profili e qualità dei payload implementati. La base della fase 3 è implementata con waterfall tool, API paginata e metriche Go condivise; le fasi restano soggette ai limiti di cattura ancora aperti. Il primo MVP analytics aggiunge catalogo storico, dashboard e trend; findings e completamento degli export restano aperti. Evidenze e limiti in [PROGRESS.md](PROGRESS.md). Vedi [direzione](PRODUCT_DIRECTION.md), [architettura](ARCHITECTURE.md), [telemetria](TELEMETRY_SPEC.md) e [analisi](ANALYSIS_SPEC.md).

Non vengono assegnate date arbitrarie. Ogni fase ha una condizione di uscita verificabile e prepara quella successiva.

## Fase 1 — Cattura passiva e migrazione esplicita

Obiettivo: registrare senza introdurre decisioni sui comandi.

Lavoro:

- Separare il percorso di cattura dal gate in hook e proxy. Il nuovo percorso non istanzia policy, broker di approvazioni o budget che possano fermare una chiamata.
- Rendere neutrali le risposte degli hook, incluso il caso collector spento. Per Claude Code preservare le permission native: nessun `allow` che le scavalchi.
- Sostituire l'attesa potenzialmente illimitata del client hook con un limite finito di raccolta; distinguere questo limite dalla durata del tool.
- Aggiornare install/uninstall affinché generino il comportamento passivo e preservino gli hook altrui.
- Togliere dal percorso principale del cockpit approvazioni, rischio e blocchi di budget, mantenendo la lettura storica dei log.

Migrazione da fissare prima del rilascio:

- Introdurre un profilo esplicito di osservazione durante lo sviluppo, con spelling CLI fissato dalla prima modifica e testato. Non presentarlo come flag già esistente.
- Usare il pivot come cambiamento di versione dichiarato: il comportamento predefinito della nuova release sarà passivo. Chi dipende dal vecchio enforcement deve poter capire che non è più offerto dal nuovo percorso e restare sulla versione precedente.
- Rifiutare con messaggio utile le vecchie opzioni di enforcement non supportate; non accettarle fingendo che proteggano ancora il progetto.
- Nessuna reinstallazione automatica o modifica degli hook reali. Fare una migrazione esplicita di un progetto di prova, poi documentare l'aggiornamento per gli utenti.
- Aggiornare `AGENTS.md`, README e `PRODUCT.md` quando il codice nuovo soddisfa i relativi check, senza lasciare il requisito `approvals: true` per un prodotto passivo. La landing page resta fuori scope.

Accettazione:

- Fixture di comandi prima bloccati vengono registrate e non rifiutate da Sentinel; usare input sintetici, senza eseguire comandi distruttivi.
- Con il collector spento l'hook termina in modo neutro entro il limite previsto; nessuna attesa o conferma aggiuntiva di Sentinel.
- Installare due volte non duplica Sentinel; uninstall rimuove solo Sentinel; tutti gli hook preesistenti restano uguali.
- Il proxy inoltra richieste e risposte invariate anche se la UI è chiusa o l'analisi non è avviata.
- Il messaggio di migrazione chiarisce la cessazione dei blocchi senza cambiare installazioni dell'utente in silenzio.

## Fase 2 — Dati correlabili e cattura affidabile

Obiettivo: costruire una base che non confonda conversazioni, tempi e payload.

Completato: envelope v2 additivo, identità di sorgente/sessione/connessione/chiamata, ID nativi tipizzati, correlazione dichiarata e filtri per sessione nel cockpit. Le chiamate indistinguibili concorrenti restano ambigue; le risposte orfane non generano richieste inventate. Durata e sua provenienza sono separate dagli intervalli osservati. Profili standard/full, redazione prima di persistenza/stream e limiti per payload dichiarati; misure della sorgente con unità separate dal JSON normalizzato/conservato, omissioni e trasformazioni visibili in inspector e report. Il replay preserva queste informazioni senza modificare vecchi log.

Errori del sink visibili e recupero dichiarato della coda JSONL implementati nel MVP analytics. Ancora aperto: capability esplicite, code e memoria totale limitate, gap e recupero dello stream. La [API profiler v1](PROFILER_API.md) versionata/paginata è implementata. Le dimensioni originali sono note solo per i valori effettivamente misurati o dichiarati dalla sorgente; full non recupera omissioni a monte. La fase non è chiusa.

Lavoro:

- Introdurre schema v2, identità di sorgente/sessione/connessione/chiamata e qualità di correlazione.
- Separare la registrazione Sentinel dalle sessioni dell'agente; supportare selezione e filtri senza mescolarle.
- Registrare provenienza dei tempi, dimensioni prima/dopo trasformazioni e capability effettive.
- Implementare profili di cattura, redazione prima della persistenza e metadati dei troncamenti.
- Rendere visibili errori del sink e gap; usare code e memoria limitate, con recupero dello stream dal log quando possibile.
- Aggiungere lettura compatibile dei JSONL precedenti e recupero dichiarato della coda incompleta.
- Fissare il contratto HTTP versionato per sessioni, chiamate, dettagli e risultati derivati, inclusa paginazione. Non cambiare implicitamente il significato dell'API precedente.

Accettazione:

- Due sessioni con lo stesso comando e due server con lo stesso ID RPC restano distinti.
- Due chiamate concorrenti non distinguibili vengono marcate ambigue; non ricevono una durata inventata.
- Output grande, redatto o omesso mantiene informazioni di completezza corrette. La vista raw distingue hook e frame MCP.
- La perdita di eventi live recuperabili è distinta dalla perdita di persistenza. Un disco in errore non blocca indefinitamente il trasporto.
- Una traccia vecchia si apre senza riscritture; campi sconosciuti non diventano zero. Una traccia interrotta dichiara la propria incompletezza.

## Fase 3 — Profiler utilizzabile

Implementato: waterfall per chiamata con unione degli intervalli, tempo non attribuito e incomplete visibili; profilo e classifiche per tool; liste paginate e dettaglio su richiesta; metriche Go condivise con replay/report; verifica con 10.000 chiamate sintetiche. Restano la suddivisione interna dei singoli payload in chunk e le garanzie di affidabilità della fase 2. Nessuno span LLM viene inventato.

Obiettivo: rendere leggibile una sessione anche senza findings automatici.

Lavoro:

- Waterfall delle chiamate con sovrapposizioni, incomplete e intervalli non attribuiti.
- Riepilogo per tool/server di esiti, distribuzione delle durate e volumi; graduatoria degli output.
- Dettaglio paginato di argomenti, risultati, patch disponibili e qualità dei dati.
- Sostituire le stime in dollari e le etichette ambigue con misure e denominatori dichiarati. Eventuali token stimati restano secondari.
- Usare un'unica implementazione delle metriche in Go per UI, replay ed export.

Accettazione:

- Chiamate parallele non gonfiano il tempo trascorso; il tempo non attribuito non viene chiamato «LLM thinking».
- Durata zero, dato mancante, errore e interruzione hanno presentazioni distinte.
- L'utente passa dalla graduatoria a una richiesta/risposta precisa e vede eventuali troncamenti.
- La UI resta navigabile su una fixture di 10.000 chiamate con payload limitati; registrare ambiente, memoria e tempi di caricamento prima di pubblicare obiettivi numerici di prestazione.

## Fase 4 — Analisi con evidenze

Obiettivo: evidenziare sequenze utili senza sostituire i dati con giudizi opachi.

Lavoro:

- Estendere `internal/analysis`, già usato per proiezione e metriche, con versioni delle regole e riferimenti stabili ai findings.
- Aggiungere R1–R4: risposte grandi, chiamate identiche, errori ripetuti, chiamate lunghe.
- Mostrare findings con dati mancanti, regola/parametri e collegamento alla selezione nella waterfall.
- Raggruppare findings sovrapposti; mantenere polling e sessioni senza anomalie tra i controesempi.

Accettazione:

- Ogni segnalazione apre tutte le evidenze necessarie e spiega perché è comparsa.
- Stesso snapshot, parametri e versione producono gli stessi risultati in live e replay.
- Le fixture includono falsi positivi plausibili, confini delle finestre temporali, gap di cattura e argomenti diversi che diventano uguali dopo la redazione: questi ultimi non devono produrre falsi finding di identità.
- Su sessioni reali, annotare quali segnalazioni hanno aiutato a scegliere un cambiamento e quali hanno solo creato rumore. Non dichiarare risparmi economici senza misure appropriate.

## Fase 5 — Post-mortem e prima release coerente

Obiettivo: consegnare un ciclo completo di registrazione, diagnosi, replay e condivisione locale.

Lavoro:

- Catalogo locale delle registrazioni e selezione delle sessioni contenute, con indice ricostruibile.
- Export JSONL ricaricabile e report HTML offline con sintesi, waterfall o intervalli equivalenti consultabili, findings ed evidenze selezionate.
- Export con selezione consapevole dei payload; il report non reintroduce contenuti esclusi dal profilo di cattura.
- Documentare setup, comportamento offline, limiti dei dati, migrazione e rimozione degli hook per la versione effettiva.
- Preparare esempi riproducibili e prove di funzionamento senza credenziali o dati privati.

Accettazione:

- Il JSONL esportato riapre la stessa sessione con la stessa analisi; il report HTML funziona senza rete o asset esterni.
- Payload con HTML/script restano testo inerte; il report indica cattura, redazioni e versione dell'analisi.
- Una demo copre un output grande, una chiamata lenta e retry falliti senza eseguire comandi pericolosi.
- Il setup nuovo non richiede che Sentinel faccia da approvatore; i controlli documentati corrispondono al binario rilasciato.

Le fasi 1–5 completano la base di raccolta, profiling e diagnosi. Il primo MVP analytics richiede anche dashboard aggregata e trend sul catalogo multi-sessione; la release analytics completa include almeno un importatore di usage nativo e il confronto A/B descritti nel piano di completamento.

## Dopo la prima release

Usage nativo e confronti controllati tra sessioni sono workstream centrali del piano analytics. Ulteriori priorità da scegliere sulla base delle sessioni raccolte: diagnostica degli schemi MCP, cicli edit/test con patch sufficienti e nuovi importatori di transcript/usage.

Restano esclusi fino a nuova decisione: chiamate a LLM per spiegare le sessioni, integrazioni cloud, dashboard di team, benchmark pubblici di agenti e modifiche alla landing page.

## Strategia di verifica durante lo sviluppo

Usare fixture JSONL e payload hook sanitizzati; testare il comportamento osservabile, soprattutto neutralità, correlazione, perdita dati e riproducibilità. Preferire un fake server MCP per guasti, concorrenza e frame grandi.

Per modifiche Go eseguire i check pertinenti del repository (`make test` include vet e race test); per il frontend aggiungere typecheck e build. Usare test mirati durante il lavoro e i check completi prima di chiudere una fase. I test di installazione operano in directory temporanee, senza toccare gli hook del progetto dell'utente.

Le modifiche documentali richiedono verifica dei link e coerenza con il codice. Le modifiche applicative richiedono anche i check sopra e una prova del percorso completo con dati sintetici, senza modificare l'installazione dell'utente.
