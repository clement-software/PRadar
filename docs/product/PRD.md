# PRD — PRadar

**Statut :** validé pour le démonstrateur

**Date :** 16 septembre 2026

**Produit :** application de bureau personnelle de compréhension des pull requests Forgejo

## Problème

Le volume de pull requests produites par des humains et des agents rend leur
compréhension coûteuse. Une liste Forgejo ou une notification indique qu'un
changement existe, mais ne permet pas de comprendre rapidement pourquoi il
existe, quel comportement il modifie ni quels risques méritent une revue.

PRadar doit permettre de comprendre une pull request en moins d'une minute,
puis de décider si une revue approfondie est nécessaire. Il assiste la revue
humaine ; il n'approuve, ne rejette et ne note aucune pull request.

## Utilisateur et acteurs

- **Utilisateur principal :** le propriétaire du produit, architecte ou tech
  lead suivant plusieurs dépôts sur une même instance Forgejo.
- **Forgejo :** source des dépôts, métadonnées, états, commits et diffs des pull
  requests.
- **Claude CLI avec `show-me` :** moteur d'analyse du MVP.
- **PRadar :** collecte les changements, ordonnance les analyses, conserve les
  résultats localement et les présente dans une timeline.

Le vocabulaire canonique du produit est défini dans `CONTEXT.md`.

## Résultats attendus

Pour la dernière version d'une pull request, l'utilisateur doit pouvoir, en
moins d'une minute :

1. expliquer pourquoi la pull request existe ;
2. identifier le comportement, le flux ou les composants qu'elle modifie ;
3. identifier les risques signalés par l'analyse ;
4. décider si une revue approfondie du diff est nécessaire.

L'importance mesure l'ampleur structurelle du changement, jamais sa pertinence
personnelle pour l'utilisateur.

## Périmètre du MVP

### Abonnements

- Une seule instance Forgejo est configurée, avec plusieurs dépôts suivis.
- L'utilisateur ajoute un dépôt par URL et PRadar valide son accès.
- Le jeton Forgejo est strictement en lecture seule et conservé uniquement dans
  le trousseau macOS.
- À l'ajout, PRadar analyse par défaut les dix pull requests ouvertes les plus
  récemment mises à jour. L'utilisateur peut choisir « aucune » ou « toutes ».
- Les pull requests en brouillon sont ignorées par défaut.
- Les pull requests créées par des bots ou des agents sont analysées ; un
  abonnement peut exclure explicitement certains auteurs.
- Se désabonner arrête la collecte mais conserve l'historique. Une action
  destructive distincte et confirmée supprime toutes les données du dépôt.

### Collecte

- PRadar interroge Forgejo toutes les cinq minutes.
- Une synchronisation complète a lieu au lancement et au réveil du poste.
- Les changements rapprochés d'une même pull request sont regroupés pendant dix
  minutes ; seule la version de tête la plus récente est ensuite analysée.
- Ouverture, réouverture, changement de tête, de titre ou de description
  déclenchent une analyse.
- Fermeture et fusion mettent à jour l'état sans relancer d'analyse. La carte
  reste visible jusqu'à sa lecture ou son archivage.
- Le MVP ne crée et ne reçoit aucun webhook.

### Analyse

- Le MVP utilise uniquement Claude CLI, un modèle global et une session
  d'analyse simultanée.
- `show-me` produit une représentation visuelle en Markdown, Mermaid,
  pseudocode et diffs ciblés. Les artefacts HTML ne sont pas acceptés.
- Chaque analyse produit le contrat versionné `pradar.analysis.v1` :
  - référence de la pull request et SHA de tête ;
  - SHA précédemment analysé, lorsqu'il existe ;
  - statut `ok` ou `unavailable` ;
  - intention en une ou deux phrases ;
  - importance structurelle `low`, `medium` ou `high` ;
  - indicateurs de risque ;
  - corps visuel de l'analyse ;
  - explication obligatoire des changements depuis la version précédente ;
  - provenance : moteur, modèle, versions du prompt et du skill, durée et usage
    lorsque disponibles.
- Le contrat ne contient ni score de pertinence ni sujets.
- L'identité d'une analyse est formée par la pull request, sa révision d'entrée,
  la version du prompt, la version du skill, le moteur et le modèle. La révision
  d'entrée comprend le SHA de tête, le titre et la description normalisés ainsi
  qu'une génération incrémentée lors d'une réouverture ; chaque déclencheur
  d'analyse possède donc une identité distincte.
- Un rejeu avec une identité différente crée une nouvelle analyse et conserve
  les analyses antérieures.
- Une analyse échouée est retentée trois fois avec un délai croissant. Après le
  troisième échec, une carte « analyse indisponible » conserve le lien Forgejo
  et permet un rejeu manuel.
- Le MVP n'impose aucun plafond quotidien. Un plafond configurable pourra être
  ajouté ultérieurement.

### Timeline et détail

- La timeline présente une seule carte par pull request, correspondant à sa
  dernière version connue, par activité décroissante.
- Une carte affiche le dépôt, le numéro et le titre, l'intention, l'auteur, la
  dernière mise à jour, l'état, l'importance, les risques et l'état de lecture.
- Les filtres du MVP sont : non lu, dépôt, état de la pull request, importance
  et risque.
- Le détail affiche l'en-tête Forgejo, l'analyse complète, les changements
  depuis la version précédente et l'historique des analyses et états.
- Les actions disponibles sont : ouvrir le détail, ouvrir dans Forgejo, marquer
  comme lu et archiver.
- Une nouvelle version remet la pull request en « non lu » et fait réapparaître
  une pull request archivée.
- La provenance technique est consultable dans une section secondaire.
- Les maquettes sous `ui-model/` donnent une direction esthétique — cartes
  pastel, contours bleus et identité PRadar — sans imposer la mise en page.
- L'interface suit le thème système clair ou sombre, respecte un contraste
  WCAG AA, expose un focus visible et permet la navigation clavier dans la
  timeline et le détail.
- Le MVP n'émet aucune notification système.

## Données, confidentialité et sécurité

- Les métadonnées, analyses, états utilisateur, travaux et éléments de
  provenance sont conservés localement.
- Les copies temporaires du code et les diffs sont supprimés après l'analyse et
  ne deviennent pas des données durables de PRadar.
- Les contenus d'une pull request sont non fiables. Le moteur s'exécute sans
  secret, avec des outils limités à la lecture, dans un espace temporaire.
- Le Markdown affiché est assaini et Mermaid utilise un mode strict.
- Un abonnement ne peut pas être actif si le moteur configuré n'est pas
  autorisé à traiter le contenu du dépôt.
- Les analyses sont conservées tant que l'utilisateur ne demande pas une
  suppression explicite. Il n'existe aucune purge automatique au MVP.
- Les mesures d'usage restent locales et ne sont jamais envoyées par
  télémétrie. Elles peuvent être exportées manuellement.

## Contraintes

- Application de bureau macOS, mono-utilisateur.
- Code de production en Go et distribution visée sous forme d'un binaire
  unique.
- Forgejo est la seule forge prise en charge au MVP.
- L'analyse s'arrête lorsque l'application est fermée ; le rattrapage au
  prochain lancement garantit la continuité.
- Le moteur d'analyse et le stockage restent des frontières remplaçables. Leur
  architecture de production n'est pas décidée par ce PRD.
- Le mécanisme de coordination locale SQLite — identité idempotente, réclamation
  atomique par bail et publication conditionnelle — est validé par le prototype
  `codex/prototype/pradar-integration-spike` au commit `2007e82`. Le driver, la
  compatibilité Forgejo réelle et l'utilité de `show-me` restent des hypothèses
  jusqu'à validation du démonstrateur complet.

## Démonstrateur vertical

Le démonstrateur doit exécuter le scénario suivant de bout en bout :

1. ajouter l'URL d'un dépôt et valider son jeton ;
2. importer les dix pull requests récentes ;
3. détecter une nouvelle version par polling et anti-rebond ;
4. créer et réclamer atomiquement un travail dans SQLite ;
5. lancer `show-me` via Claude CLI ;
6. valider et enregistrer `pradar.analysis.v1` ;
7. afficher la carte et le détail dans un visualiseur minimal jetable ;
8. marquer la pull request comme lue puis l'archiver ;
9. faire réapparaître la pull request, non lue, après un nouveau commit.

Le visualiseur est suffisamment réaliste pour évaluer cartes, Markdown et
Mermaid, mais il ne préjuge pas du toolkit graphique de production.

### Corpus et protocole

- Vingt pull requests réelles sont sélectionnées à l'avance dans deux ou trois
  dépôts Forgejo.
- Le corpus mélange petites et grosses pull requests, contributions humaines et
  générées par agent, ainsi que code, CI et infrastructure.
- Les SHA de tête sont figés avant l'évaluation.
- Il n'existe ni comparaison A/B ni évaluation à l'aveugle.
- L'utilisateur mesure le temps puis vérifie l'intention, le changement
  structurel, les risques, la capacité à décider d'une revue et l'absence
  d'erreur factuelle critique.

### Critère de sortie

Au moins seize analyses sur vingt doivent permettre le résultat attendu en
moins d'une minute, sans erreur factuelle critique. Si ce seuil n'est pas
atteint, l'analyse ou sa présentation est corrigée puis le corpus est réévalué
avant de poursuivre la construction du produit.

## Validation en usage réel

Après le démonstrateur, PRadar est utilisé quotidiennement pendant deux
semaines. Les signaux locaux sont :

- temps médian nécessaire à la compréhension ;
- proportion d'analyses jugées utiles ;
- nombre d'erreurs factuelles critiques ;
- proportion de pull requests comprises sans ouverture immédiate du diff
  Forgejo ;
- échecs, rejeux et délais entre une nouvelle version et sa carte.

## Hors périmètre

Sont explicitement reportés après le MVP :

- pertinence personnalisée et vue « Pour moi » ;
- fils et taxonomie de sujets transverses ;
- adaptateur Claude API et choix du moteur ou du modèle par abonnement ;
- webhooks ;
- artefacts HTML ;
- plusieurs instances Forgejo ;
- processus daemon, icône système et analyses après fermeture de la fenêtre ;
- notifications système ;
- Linux et Windows ;
- recherche plein texte ;
- clone, checkout, worktrees et ouverture dans un IDE ;
- télémétrie distante, partage et fonctions multi-utilisateur ;
- lecteurs d'écran au-delà des primitives d'accessibilité fournies par le
  toolkit retenu.

## Questions ouvertes

Ces questions sont résolues par le démonstrateur ou la phase de conception, pas
par hypothèse dans le PRD :

1. `show-me` atteint-il le seuil de compréhension sur le corpus réel ?
2. SQLite convient-il à la file de travaux et à la lecture concurrente exigées
   par la tranche verticale ?
3. Quelle version de Forgejo et quels dépôts composent le corpus reproductible ?
4. Quel toolkit graphique Go affiche correctement Markdown, Mermaid, les diffs
   et les états de focus sur macOS ?
5. Quelle architecture applicative et quelle stratégie d'injection donnent les
   propriétaires et cycles de vie les plus explicites sans sur-conception ?
