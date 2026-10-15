# Agent Sentinel — nuova direzione

Decisione di prodotto del 15 ottobre 2026: **Agent Sentinel diventa un profiler locale e un flight recorder per coding agent e tool MCP.** Il valore principale è capire cosa è successo, dove è stato speso tempo e quali sequenze meritano attenzione.

Questi documenti definiscono lo sviluppo del pivot. La prima fase di cattura passiva è implementata sul branch `telemetry-profiler`; il resto rimane una specifica di destinazione. Lo stato verificato è in [PROGRESS.md](PROGRESS.md). Configurazioni installate e landing page restano invariati.

## Percorso di lettura

1. [Direzione di prodotto](PRODUCT_DIRECTION.md): destinatari, promessa, confini e prima esperienza utile.
2. [Architettura](ARCHITECTURE.md): stato reale del repository e architettura di destinazione.
3. [Contratto di telemetria](TELEMETRY_SPEC.md): identità, correlazione, tempi, payload e qualità dei dati.
4. [Specifiche di analisi](ANALYSIS_SPEC.md): metriche e diagnosi riproducibili, con evidenze e limiti.
5. [Roadmap di implementazione](ROADMAP.md): ordine dei lavori, migrazione e criteri di accettazione.

## Autorità e stato dei documenti

- Queste specifiche definiscono il **comportamento futuro desiderato**. Il vecchio blueprint in `ARCHITECTURE.md` è stato sostituito: conteneva anche ipotesi e comandi mai implementati.
- Il [README](../README.md), [PRODUCT.md](../PRODUCT.md) e il [playbook di setup](../AGENTS.md) descrivono il comportamento osservativo ora implementato e la migrazione esplicita dalle release di enforcement.
- Le indicazioni operative e i vincoli del repository restano applicabili. Queste specifiche non autorizzano a cambiare installazioni o hook dell'utente.
- `cockpit.png` documenta l'interfaccia precedente; `profiler.png` mostra la prima fase passiva con dati sintetici.
- Schema v2 e identità/correlazione sono implementati nel sottoinsieme dichiarato in TELEMETRY_SPEC.md e PROGRESS.md; le API versionate, la qualità completa della cattura e il motore di findings restano contratti da implementare.
- Il nome Agent Sentinel rimane quello del progetto. Non è previsto un lavoro su nome, branding o landing page in questa fase.

## Decisioni già prese

- Osservazione passiva come comportamento del nuovo prodotto: niente decisioni sui comandi, attese di approvazione o blocchi di budget introdotti da Sentinel.
- I permessi e le conferme del coding agent restano di sua competenza. Una risposta neutra di Sentinel non deve aggirarli.
- Un binario Go con UI incorporata; registrazione e analisi locali, senza account o servizi esterni richiesti.
- Prima release concentrata su durata dei tool, dimensione degli output, errori e ripetizioni verificabili.
- Nessuna promessa di leggere i pensieri dell'agente, ricostruire il contesto completo o conoscere la fattura del modello dai soli hook.
- Analisi deterministica e offline nella prima release. Eventuali integrazioni LLM o provider sono lavoro successivo e richiedono una specifica propria.
- Conservare la lettura delle tracce precedenti; non trasformare vecchi dati mancanti in misure inventate.

## Cosa verificare prima di scrivere codice

Ogni modifica deve chiarire quale dato osservabile usa, cosa succede quando quel dato manca e come il risultato viene verificato. In caso di conflitto tra una diagnosi accattivante e una misura affidabile, prevale la seconda.
