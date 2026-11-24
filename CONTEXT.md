# OwnWave

A self-hosted music server that scans a music library, works out each song's genre and builds radio-style stations and playlists from it.

## Language

### Library

**Release**:
One published album or single as a whole, identified by its MusicBrainz release when known, otherwise by its folder and album title.
_Avoid_: album (when the release is meant), record

**Compilation**:
A release whose songs come from several artists, such as a "Various Artists" album.
_Avoid_: sampler, mix (unless it is a DJ mix)

### Genres

**Genre**:
A broad kind of music a listener would pick a station by, such as House, Techno, Pop or Metal. Every style belongs to exactly one genre.
_Avoid_: station genre, family, category

**Style**:
A narrower kind of music within one genre, such as Deep House under House or Black Metal under Metal.
_Avoid_: subgenre, substyle

**Fence**:
The set of genres a song may still end up in, narrowed by weak hints such as a broad tag. Nothing outside a song's fence can become its genre.
_Avoid_: constraint, filter, allowed genres

**Genre tag**:
The genre text written in a music file's own tags, such as "Techno" or "Pop/Rock".
_Avoid_: file genre, ID3 genre

**Broad tag**:
A genre tag that names only a wide area (such as "Electronic", "Pop/Rock" or "Dance") and does not pick out a single genre.
_Avoid_: generic tag

**Correction**:
An admin's choice of genre for one song or for all of an artist's songs. It beats every other source.
_Avoid_: override, manual genre

**Trusted label**:
A genre known well enough to teach the classifier: a correction, or a genre tag that names exactly one genre. Broad tags and untagged songs are never trusted labels.
_Avoid_: ground truth, training tag

**Strong source**:
A source that names one specific genre or style for a song: a correction, a specific genre tag, a Discogs style, or a MusicBrainz recording or album genre.
_Avoid_: primary source, authoritative source

**Weak source**:
A source that only narrows a song's fence and never picks its genre: a broad tag, the artist's genres, or Last.fm tags.
_Avoid_: hint, secondary source

**Classifier**:
The part that guesses a song's genre from how it sounds, by comparing it with similar-sounding songs that have trusted labels. It only decides when no strong source does.
_Avoid_: audio model, AI, genre model

**Unsure song**:
A song whose sources do not agree well enough to pick a genre. It joins no genre station until an admin reviews it.
_Avoid_: unknown, unclassified
