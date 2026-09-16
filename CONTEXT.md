# PRadar

PRadar aide un utilisateur à surveiller des pull requests Forgejo et à comprendre rapidement les changements qui méritent une lecture approfondie.

## Langage

**Utilisateur principal**:
La personne qui utilise PRadar pour suivre et comprendre les pull requests de plusieurs dépôts. Pour le MVP, il s'agit du propriétaire du produit lui-même.
_À éviter_: Équipe, organisation, administrateur

**Analyse**:
Une explication structurée de l'intention, des changements importants et des risques d'une version de pull request, destinée à être comprise en moins d'une minute. Elle permet de décider si une revue approfondie est nécessaire, sans approuver, rejeter ou noter la pull request.
_À éviter_: Revue automatique, validation, verdict

**Instance Forge**:
Le serveur Forgejo qui héberge les dépôts suivis. Le MVP utilise une seule instance Forge.
_À éviter_: Forge, fournisseur Git

**Abonnement**:
Le choix de suivre les pull requests d'un dépôt d'une instance Forge. Un abonnement ne peut être actif que si le dépôt est accessible et que le moteur d'analyse configuré est autorisé à traiter son contenu.
_À éviter_: Watch, notification

**Version de pull request**:
L'état d'une pull request identifié par la révision de tête observée. Une nouvelle révision constitue une nouvelle version à comprendre, même si la pull request conserve le même titre ou le même numéro.
_À éviter_: Pull request distincte, nouvelle PR

**Anti-rebond**:
La période pendant laquelle PRadar regroupe les changements rapprochés d'une même pull request avant d'analyser sa version la plus récente.
_À éviter_: Délai d'analyse, polling

**Lu**:
État indiquant que l'utilisateur a pris connaissance de la dernière version analysée d'une pull request. Une nouvelle version remet automatiquement la pull request en état non lu.
_À éviter_: Traité, archivé

**Archivé**:
État retirant une pull request de la timeline active sans supprimer son historique. Une nouvelle version analysée la fait réapparaître en état non lu.
_À éviter_: Supprimé, fermé

**Analyse indisponible**:
État visible d'une version de pull request dont l'analyse a échoué après les tentatives automatiques. La pull request et son lien Forgejo restent accessibles et l'utilisateur peut relancer l'analyse.
_À éviter_: Pull request ignorée, analyse absente

**Carte**:
La représentation, dans la timeline, de la dernière version connue d'une pull request. Une pull request possède au plus une carte visible, même lorsque plusieurs versions ont été analysées.
_À éviter_: Analyse, notification, version

**Historique de pull request**:
La suite ordonnée des analyses et changements d'état d'une même pull request, accessible depuis sa carte.
_À éviter_: Fil de sujet, timeline

**Timeline**:
La vue principale ordonnée par activité récente qui présente une carte par pull request non archivée. Elle peut être filtrée par état de lecture, dépôt, état de la pull request, importance ou risque.
_À éviter_: Historique de pull request, liste Forgejo

**Compréhension**:
La capacité à expliquer en moins d'une minute pourquoi une pull request existe, quel comportement ou flux elle modifie, quels risques elle présente et si une revue approfondie est nécessaire.
_À éviter_: Résumé, validation, score de pertinence

**Importance**:
L'ampleur structurelle d'un changement : local et isolé, réparti sur plusieurs composants, ou touchant un contrat, l'architecture ou la sécurité. Elle ne mesure pas l'intérêt personnel de l'utilisateur.
_À éviter_: Pertinence, priorité

**Risque**:
Un aspect du changement qui demande une attention particulière lors d'une éventuelle revue humaine, par exemple la sécurité, un contrat cassant, une migration ou l'infrastructure.
_À éviter_: Erreur certaine, verdict

**Rejeu**:
La création d'une nouvelle analyse pour une version de pull request déjà analysée, après modification du prompt, du skill, du moteur ou du modèle. Le rejeu conserve les analyses antérieures dans l'historique.
_À éviter_: Réessai, remplacement

**Désabonnement**:
L'arrêt de la collecte des changements d'un dépôt sans suppression de son historique. L'effacement des données est une action distincte, destructive et confirmée.
_À éviter_: Suppression, archivage
