# Piano di completamento di Agent Sentinel

Proposta del 16 ottobre 2026, basata sul checkout `telemetry-profiler` a `1389eab` e sull’analisi incollata dall’utente. Questo documento propone lavoro futuro: non dichiara nuove funzionalità implementate e non sostituisce le specifiche esistenti.

Aggiornamento implementativo: il primo MVP catalogo/dashboard/trend è ora disponibile; stato e limiti correnti in [PROGRESS.md](PROGRESS.md) e [ANALYTICS_API.md](ANALYTICS_API.md). La ricognizione sotto rimane riferita al commit iniziale.

Correzione UX successiva dell’utente: focus principale sull’analisi delle chiamate, Timeline in home e statistiche generali nella vista dedicata Statistics. La piattaforma mantiene catalogo e metriche storiche; cambia la gerarchia di navigazione.

## Obiettivo di prodotto confermato

**An analytics platform for monitoring and analyzing AI agent usage and metrics.**

Direzione confermata dall’utente il 16 ottobre 2026. Il valore centrale è monitorare utilizzo e andamento degli agenti tra sessioni, progetti e periodi, individuare anomalie e confrontare metriche. Recorder, profiler e inspector costituiscono la base di raccolta e approfondimento della piattaforma.

Il precedente piano rinviava troppo la dimensione analytics. Storico aggregato, dashboard e trend entrano nella prima release analytics; usage nativo e confronti sono workstream espliciti del prodotto. Le garanzie di cattura si sviluppano a supporto di queste funzioni, senza trasformare l’intero hardening del recorder in un blocco preliminare a ogni funzionalità visibile.

## Ordine di consegna orientato alla piattaforma

1. **Fondazione multi-sessione:** catalogo persistente ricostruibile, progetto/agente quando noti, elenco sessioni, filtri temporali e stato di qualità. Affiancare salute del sink e recupero dei log interrotti. Uscita: riavviando Sentinel si ritrovano le sessioni e si possono aggregare senza confonderne le identità.
2. **Analytics overview:** pagina iniziale con chiamate, sessioni osservate, error rate, durate valide e volumi; trend giornalieri e breakdown per agente/progetto/tool. Filtri 24 ore/7/30 giorni, periodo precedente comparabile e drill-down fino alle chiamate. Uscita: una variazione nel grafico è spiegabile dai dati sottostanti. I periodi non coperti restano distinguibili da zero attività. Questa consegna segna il primo MVP analytics.
3. **Diagnostica e analisi dei volumi:** findings R1–R4, distribuzioni, concentrazione degli output e letture ripetute osservate, collegati alle evidenze. Uscita: dalla dashboard si identifica e si approfondisce un pattern ricorrente anche su più sessioni, mantenendo le regole nel loro scope corretto.
4. **Usage intelligence:** primo importatore opt-in di usage nativo, con modello quando noto, token input/output/cache e deduplicazione. Costi riportati dalla sorgente o stime da listino datato sono distinti. Uscita: usage storico collegato alla sua provenienza; i dati assenti non sono sostituiti da stime sui byte dei tool.
5. **Confronto di sessioni e report:** selezione A/B, differenze assolute e percentuali con denominatori validi, condizioni e copertura dichiarate; export dell’analisi e JSONL ricaricabile. Uscita: confronto riproducibile e condivisibile offline. Non serve un runner che rilanci gli agenti.
6. **Chiusura della release analytics:** hardening di code/memoria e accesso da disco completato, prove prolungate, verifica degli adattatori e documentazione. Heatmap annuale e aggregazioni settimanali estendono la navigazione quando lo storico lo permette.

La prima release analytics completa comprende storico, dashboard/trend, diagnostica, almeno una sorgente di usage nativo e confronto A/B; il primo MVP è già utilizzabile al punto 2. La copertura degli agenti è incrementale e dichiarata. Un eval runner attivo, cloud e collaborazione di team richiedono decisioni separate.

Le tranche tecniche sotto descrivono dipendenze e criteri di accettazione da distribuire tra queste consegne; la loro numerazione non impone di completare tutto l’hardening prima di iniziare catalogo e dashboard.

## Stato verificato del branch

- Checkout inizialmente pulito; branch locale e remoto `telemetry-profiler` allo stesso hash `1389eab3c255c9484b87f91834df45b4635af95a`, verificato con `git ls-remote`.
- `master` locale e remoto a `89fd7628fa6dc657ddf84504ca94797d3dc6ce86`; il branch è avanti di 34 commit, senza commit di master mancanti. Diff: 86 file, 6.399 inserimenti e 2.313 cancellazioni.
- Cattura passiva, schema v2, identità/correlazione, profili standard/full, redazione e qualità dei payload implementati.
- Proiezione e metriche Go, timeline compatta con zoom e tracce riutilizzabili, profilo tool, classifiche e API paginata implementati.
- Replay JSONL di una registrazione e report HTML offline implementati. Il selettore attuale distingue sessioni/connessioni dentro la registrazione aperta: non è un catalogo storico.
- Findings R1–R4, catalogo, aggregazioni storiche, usage nativo e confronto fra sessioni assenti.
- `make test` passato: vet, race test Go, typecheck e nove test frontend. Primo tentativo impedito dalla sandbox sulle porte localhost di `httptest`; riesecuzione consentita fuori sandbox passata.
- `cd web && npm run build` passato. Nessuna nuova verifica manuale del cockpit o delle versioni correnti degli agenti esterni in questa ricognizione.

Le verifiche riguardano il checkout, non il binario installato. Nessuna installazione, migrazione o modifica degli hook reali eseguita.

## Gap concreti nel codice

1. `internal/recorder/recorder.go`: `events` cresce per tutta la registrazione; `RecordHook` scrive il sink sotto mutex e ignora l’errore di `Encode`. Un sink lento può rallentare la cattura e il percorso di inoltro MCP.
2. Lo stesso recorder scarta gli eventi dei subscriber pieni senza un protocollo esplicito di recupero. Perdita del live e perdita della persistenza vanno distinte.
3. `internal/recorder/replay.go`: una riga JSON finale incompleta fa fallire l’intero caricamento; manca il recupero dichiarato del prefisso valido.
4. `internal/server/profiler.go`: ogni nuova revisione copia gli eventi e ricostruisce la proiezione; i cursori precedenti ricostruiscono un prefisso. La paginazione riduce il trasferimento al browser, non il costo totale sul server.
5. `internal/proxy/stream.go`: lettura di frame tramite `ReadBytes`, senza limite di riga. Il limite sui payload conservati non limita l’input né la memoria transitoria del proxy.
6. Le capability per sorgente non sono esplicite. `projectRef` esiste quando arriva un cwd, ma non garantisce un progetto per ogni evento o connessione MCP. Mancano inoltre alcuni denominatori/coperture previsti dall’analisi.

## Cosa adottare e cosa correggere nell’analisi OmniRoute

OmniRoute offre riferimenti utili di prodotto, ma opera anche sul traffico verso i modelli. Sentinel oggi osserva hook e tool MCP: la disponibilità dei dati è diversa.

- **Storico:** adottare catalogo, filtri e trend. Prima rendere navigabile e riapribile l’archivio; la heatmap viene dopo. Dati non registrati o rimossi non equivalgono a giorni senza attività.
- **Findings:** adottare subito nella prima release, dopo le garanzie minime di cattura. Le regole hanno già un contratto preciso in [ANALYSIS_SPEC.md](ANALYSIS_SPEC.md); non serve reinventarle.
- **Context footprint:** adottare quote dei byte osservati/conservati con unità e copertura. Non chiamarle occupazione del contesto: non sappiamo quali contenuti arrivino effettivamente al modello o vengano compattati.
- **Cache health:** il codice OmniRoute usa contatori `cacheRead` e `cacheCreation` dei log delle chiamate modello. I byte dei tool non permettono di ricostruirli né di identificare con certezza chi abbia invalidato la cache.
- **Riletture di file:** partire da “letture ripetute osservate”. “Stesso file invariato” richiede contenuto confrontabile e copertura delle modifiche; l’assenza di un edit hook non prova che nessuno abbia modificato il file.
- **Diff efficiency:** rinviare il punteggio unico. Il diff finale non esiste nei dati attuali e non misura da solo qualità o utilità. Servirebbe uno snapshot Git esplicito con stato iniziale/finale e gestione delle modifiche esterne.
- **Token e costi:** privilegiare usage nativo importato, con provenienza e deduplicazione. Un tokenizer locale può stimare soltanto il testo disponibile e va validato per modello; un tokenizer OpenAI non è automaticamente valido per Claude. Tokenizzare output dei tool non ricostruisce i token fatturati della sessione.
- **Confronti ed eval:** confrontare prima sessioni registrate con condizioni dichiarate. L’eval runner OmniRoute esegue chiamate reali e valuta output: riprodurlo introdurrebbe un nuovo prodotto attivo, distinto dal replay passivo di Sentinel.

Correzioni delle regole rispetto al testo incollato:

- R1: **almeno 64 KiB di testo osservato**, non JSON conservato o frame indiscriminatamente; indicare limiti inferiori quando appropriato.
- R2: **almeno quattro chiamate completate identiche in cinque minuti**, nello stesso scope; polling non significa automaticamente stagnazione.
- R3: la specifica attuale richiede un gruppo R2 con **almeno tre errori**; non equivale a tre fallimenti consecutivi. “Stesso errore” richiede una firma verificabile. R3 incorpora R2 per evitare avvisi duplicati.
- R4: **almeno dieci secondi di durata valida**, con provenienza. È una chiamata lunga, non una prova di collo di bottiglia o timeout.
- Per R2/R3, argomenti redatti o troncati non provano l’identità originale: regola non applicabile, senza hash nascosti dei segreti.

## Tranche tecniche a supporto delle consegne

Le unità seguenti sono tranche implementative, da dividere ulteriormente se una PR non resta revisionabile. I nomi dei nuovi package ed endpoint sono proposte, non API esistenti.

### 1. Rendere visibili persistenza e incompletezza

Ambito: recorder, replay, configurazione server e metadati di qualità.

- Esporre salute del sink, ultimo errore e ultimo evento scritto con successo, distinguendo scrittura da garanzia di durabilità.
- Rendere osservabili le perdite dei subscriber e la possibilità di recupero; non affidare la diagnostica soltanto allo stesso sink guasto.
- Recuperare l’ultima riga incompleta solo a EOF, preservando il prefisso valido e dichiarando l’incompletezza; una riga corrotta nel mezzo resta un errore.
- Dichiarare capability effettive e copertura: esiti, durata, argomenti, output, modifiche e identità disponibili. Capacità della sorgente e disponibilità del singolo evento restano distinte.

Uscita: fixture di sink in errore, subscriber lento e JSONL interrotto producono uno stato distinguibile; nessuna sessione danneggiata appare completa. Hook sempre neutrali.

### 2. Isolare l’I/O e limitare la memoria

Ambito: recorder, trasporto proxy, tracker delle chiamate, accesso ai dati del profiler.

- Coda asincrona limitata per eventi già redatti, con sequenza assegnata in ordine; limite anche in byte, non solo nel numero di elementi.
- Saturazione: inoltro dei tool invariato e perdita di telemetria dichiarata, senza attesa illimitata. Flush e arresto con scadenza, contatori di perdita indipendenti dalla coda piena.
- Store consultabile da disco con cache limitata; limitare anche proiezioni, indici in RAM, gruppi di correlazione aperti e prefissi dei cursori. Sostituire soltanto `events` con un ring buffer romperebbe il contratto dei cursori.
- Snapshot paginati riproducibili: consultare il prefisso persistito quando disponibile; per revisioni non recuperabili, restituire uno stato esplicito invece di ricalcolare da un insieme parziale.
- Se `--log -` disabilita la persistenza, dichiarare finestra di conservazione e revisioni scadute: nessuna promessa di recupero da disco.
- Separare inoltro MCP e accumulo per l’osservazione dei frame grandi; superata la capacità di cattura, continuare a inoltrare byte integri e dichiarare il contenuto non osservato. Preservare ordine e correlazione verificabile.

Uscita: fault injection con sink lento/guasto, coda piena, frame grande e lunga registrazione; memoria sotto limiti dichiarati e misurati nell’ambiente di prova. Nessuna crescita proporzionale a tutti i payload, nessuna perdita silenziosa o modifica dei frame. Questa tranche chiude il nucleo di affidabilità della fase 2.

### 3. Completare i denominatori delle metriche

Ambito: `internal/analysis`, API, profilo tool e report.

- Successi, errori, esiti sconosciuti e interruzioni separati; error rate = `error / (ok + error)`, non errori su tutte le richieste.
- Campioni di latenza inclusi/esclusi e relativa motivazione; aggregazioni per server quando identificabile.
- Totali per unità coerenti di byte osservati e conservati, quantità note/ignote e copertura. Non sommare UTF-8, JSON e frame come se fossero la stessa metrica.
- Definire la lettura per porzioni dei dettagli grandi e la ricerca senza caricare ogni payload nel browser, oppure dichiarare esplicitamente il limite per la prima release.

Uscita: stessi denominatori, esclusioni e valori in live, replay e report; i dati mancanti non diventano zero. Le metriche rimangono in Go.

### 4. Implementare findings deterministici R1–R4

Ambito: estensione di `internal/analysis` e contratto HTTP.

- Definire `Finding`, parametri/versione, riferimenti alle evidenze, condizioni di applicabilità e ID deterministici per lo stesso snapshot.
- Implementare prima R1/R4, poi R2/R3 con firma canonica, finestre mobili, confini di sessione e soppressione delle duplicazioni.
- Esporre elenco paginato dei findings, ancorato a registrazione/revisione/scope, e dettaglio delle evidenze. Preferire una risorsa dedicata, ad esempio `/api/v1/findings`, per non gonfiare il polling delle chiamate.
- Distinguere nessun superamento di soglia, regola non applicabile e cattura incompleta.

Uscita: fixture per Unicode, soglie esatte, polling, argomenti redatti, concorrenza, sessioni distinte, gap ed errori diversi. Stessi risultati in live, replay ed export a parità di snapshot, regole e parametri.

### 5. Chiudere il percorso diagnostico nel cockpit

Ambito: vista Findings, inspector, selezione della timeline e report.

- Ordine diagnostico: errori ripetuti, output grandi, chiamate lunghe, ripetizioni senza errore.
- Ogni finding mostra osservazione, regola, soglia, qualità e dati mancanti; apre le chiamate coinvolte.
- Gestire evidenze su pagine diverse: navigazione a ciascuna chiamata e selezione del gruppo fissata alla revisione. Un semplice highlight della pagina corrente non è sufficiente.
- Dal dettaglio della chiamata rendere raggiungibili i findings associati.
- Includere findings e riferimenti consultabili nel report offline, senza eseguire contenuti dei payload.

Uscita: percorso finding → evidenze → timeline → dettaglio, anche tra pagine e su mobile; regressioni su filtri e scope. Revisione manuale di sessioni sanitizzate per individuare rumore e falsi positivi.

### 6. Catalogo locale e pacchetto post-mortem

Ambito: nuovo catalogo locale, apertura registrazioni ed export.

- Indice ricostruibile dai JSONL come fonte primaria. Iniziare con un indice locale semplice/versionato; introdurre SQLite soltanto se misure di scala o query lo giustificano.
- Distinguere registrazione Sentinel, sessioni native e connessioni. Filtri per progetto noto, agente, data e stato completo/incompleto; gruppi espliciti per metadati sconosciuti.
- Apertura di una registrazione dal cockpit, con invalidazione corretta di cursori e selezioni. Supportare file attivi e percorsi scelti esplicitamente, oltre alla directory predefinita.
- Export JSONL ricaricabile; selezione esplicita dell’ambito, profilo di cattura e payload inclusi. Report HTML con findings, evidenze selezionate e qualità, usando solo dati già disponibili.
- Il report deve funzionare senza rete. HTML/script catturati restano testo inerte; nessun recupero retroattivo dei payload esclusi.

Uscita: riavvio → catalogo → vecchia sessione → stesso finding → report e JSONL riaperti con valori coerenti. Indice cancellato/ricostruito senza perdere le registrazioni; cache invalidate se un log attivo cresce.

### 7. Verifica della release e documentazione

Ambito: test completi, fixture/demo e documenti operativi.

- Eseguire `make test`, build frontend e smoke completo con binario temporaneo e payload sintetici.
- Verificare una versione dichiarata di Cursor e Claude Code: hook reali disponibili, caricamento, campi e limiti effettivi. I test degli adattatori non sostituiscono questa verifica.
- Provare registrazione prolungata, guasti, replay e report; misurare ambiente, RSS, tempi e dimensioni. La prova già documentata da 10.000 chiamate non dimostra memoria limitata.
- Aggiornare README, PRODUCT, PROGRESS, API e setup alla release effettiva. Correggere già in fase di pianificazione editoriale le frasi obsolete di `docs/README.md`, che presentano l’API versionata come ancora da implementare.
- Migrazione osservativa esplicita e prova in un progetto temporaneo; nessuna sostituzione automatica dei binari referenziati dagli hook.

Uscita: una demo riproducibile copre output grande, chiamata lunga, errori ripetuti, riavvio e report offline. Limiti e capability reali sono pubblicati; nessuna promessa di risparmio o di successo del task dedotta dalle sole chiamate.

## Workstream analytics centrali

### A. Storico aggregato e trend

Dipendenza: catalogo e qualità affidabili. Viste 24 ore/7/30 giorni, poi heatmap annuale. Aggregare chiamate, esiti, durate valide e volumi con i rispettivi denominatori. Documentare fuso orario e attribuzione ai giorni; l’attività mancante non è zero. Deduplicare lo stesso log copiato e non fondere due registrazioni diverse senza identità provata. Non sommare percentili né intervalli sovrapposti per ricavare tempo attivo globale.

### B. Volumi osservati e letture ripetute

Dipendenza: metriche di volume e capability. Mostrare concentrazione per tool e richieste/risposte, con limiti inferiori e copertura. Un primo report può dire “questo tool rappresenta il 70% del testo osservato noto”, senza attribuirgli il 70% del contesto. Le letture ripetute restano un fatto descrittivo; l’invarianza del contenuto richiede evidenze ulteriori compatibili con il profilo di cattura.

### C. Usage nativo, poi cache health e scenari economici

Dipendenza: un importatore esplicito per una sorgente alla volta. Fissare consenso di lettura, formato/versione, redazione, modello noto, identità del messaggio, valori incrementali/cumulativi e deduplicazione con eventuali eventi hook.

Input/output/cache read/cache write nativi restano distinti. Mostrare cache health solo con contatori adeguati e copertura dichiarata, senza dedurre causalità dal volume dei tool. Le stime token sul testo sono secondarie e separate dai contatori nativi. Un eventuale scenario di costo richiede modello, listino datato e unità appropriate; non è la fattura né una proiezione attendibile ottenibile dai soli byte catturati.

### D. Confronto di sessioni, poi eval opzionali

Dipendenza: catalogo e metadati dichiarati del task. Confronto A/B di chiamate, esiti, tempi osservati e volumi; indicare differenze di copertura, agente/modello/configurazione e stato iniziale. Qualità del risultato solo se esistono test o rubriche esplicite. Non dichiarare un vincitore dal minor numero di chiamate.

Un runner di eval su task fissi richiederà un progetto separato: esecuzione autorizzata, ambiente isolato, revisioni iniziali, test di successo, ripetizioni e costi. Non è un’estensione implicita del replay, che non esegue comandi.

## Fonti e limiti della ricognizione

Fonti locali: [PROGRESS.md](PROGRESS.md), [ROADMAP.md](ROADMAP.md), [TELEMETRY_SPEC.md](TELEMETRY_SPEC.md), [ANALYSIS_SPEC.md](ANALYSIS_SPEC.md), [PROFILER_API.md](PROFILER_API.md) e i file implementativi citati sopra.

Riferimenti OmniRoute verificati sul ref `release/v3.8.52` il 16 ottobre 2026:

- [Cache health](https://github.com/diegosouzapw/OmniRoute/blob/release/v3.8.52/src/lib/usage/cacheHealth.ts): contatori cache, distribuzione e concentrazione per modello.
- [Usage history](https://github.com/diegosouzapw/OmniRoute/blob/release/v3.8.52/src/lib/usage/usageHistory.ts): storico persistito e contatori usage.
- [Analytics UI](https://github.com/diegosouzapw/OmniRoute/blob/release/v3.8.52/src/app/%28dashboard%29/dashboard/analytics/page.tsx): viste Evals e Cache Health.
- [Framework Evals](https://github.com/diegosouzapw/OmniRoute/blob/release/v3.8.52/docs/frameworks/EVALS.md): suite, rubriche, persistenza e orchestrazione di chiamate reali.
- [API reference](https://github.com/diegosouzapw/OmniRoute/blob/release/v3.8.52/docs/reference/API_REFERENCE.md): endpoint usage, cache-health ed evals.

Non è stato eseguito OmniRoute né svolto un audit completo del suo codice. Non è stata verificata la specifica heatmap annuale menzionata nel testo incollato; resta un’idea di interfaccia, non una dipendenza tecnica del piano.
