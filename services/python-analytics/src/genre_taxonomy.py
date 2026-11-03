"""Station genres and how every genre source maps onto them.

A station genre is one of the broad genres OwnWave builds stations from.
Every source (the Discogs400 audio model, file tags, MusicBrainz artist
genres) speaks its own vocabulary; this module translates each one into a
distribution over station genres, so they can be blended.

Discogs400 labels are mapped by their full "Parent---Sub" name. Mapping by
parent alone would put about a third of the model's labels into one
"Electronic" bucket.
"""

import re
from typing import Dict, List, Optional

# Station genre id -> display name. Order is the display order.
STATION_GENRES: Dict[str, str] = {
    "techno": "Techno",
    "house": "House",
    "trance": "Trance",
    "drum_and_bass": "Drum & Bass",
    "bass": "Bass & Breaks",
    "hard_dance": "Hard Dance",
    "dance": "Dance & Disco",
    "synth": "Synth & Electro",
    "ambient": "Ambient & Chill",
    "experimental": "Experimental",
    "hip_hop": "Hip Hop",
    "soul": "Soul, Funk & R&B",
    "pop": "Pop",
    "rock": "Rock",
    "indie": "Indie & Alternative",
    "metal": "Metal",
    "punk": "Punk & Hardcore",
    "jazz": "Jazz",
    "blues": "Blues",
    "classical": "Classical",
    "folk": "Folk & Country",
    "latin": "Latin",
    "reggae": "Reggae",
    "world": "World",
    "soundtrack": "Soundtrack",
    "other": "Other",
}

# Genres that are never offered as stations.
NON_STATION_GENRES = frozenset({"other"})

# Electronic station genres; a generic "electronic" tag spreads over these.
ELECTRONIC_GENRES = (
    "techno",
    "house",
    "trance",
    "drum_and_bass",
    "bass",
    "hard_dance",
    "dance",
    "synth",
    "ambient",
    "experimental",
)

# --- Discogs400 -------------------------------------------------------------

_DISCOGS_PARENT_DEFAULT = {
    "Blues": "blues",
    "Brass & Military": "other",
    "Children's": "other",
    "Classical": "classical",
    "Electronic": "experimental",
    "Folk, World, & Country": "world",
    "Funk / Soul": "soul",
    "Hip Hop": "hip_hop",
    "Jazz": "jazz",
    "Latin": "latin",
    "Non-Music": "other",
    "Pop": "pop",
    "Reggae": "reggae",
    "Rock": "rock",
    "Stage & Screen": "soundtrack",
}

_DISCOGS_OVERRIDES = {
    "Electronic": {
        "techno": [
            "Acid", "Deep Techno", "Dub Techno", "Hard Techno", "Minimal",
            "Minimal Techno", "Schranz", "Techno", "Bleep", "Tribal",
        ],
        "house": [
            "Acid House", "Deep House", "Electro House", "Euro House", "Garage House",
            "Ghetto House", "Hard House", "Hip-House", "House", "Italo House",
            "Progressive House", "Tech House", "Tribal House", "Tropical House", "Beatdown",
        ],
        "trance": [
            "Goa Trance", "Hard Trance", "Progressive Trance", "Psy-Trance", "Tech Trance", "Trance",
        ],
        "drum_and_bass": ["Drum n Bass", "Jungle"],
        "bass": [
            "Bassline", "Big Beat", "Breakbeat", "Breaks", "Broken Beat", "Dubstep",
            "Grime", "Halftime", "Juke", "Ghetto", "Progressive Breaks", "Speed Garage",
            "UK Garage",
        ],
        "hard_dance": [
            "Donk", "Gabber", "Happy Hardcore", "Hardcore", "Hardstyle", "Jumpstyle",
            "Makina", "Speedcore",
        ],
        "dance": [
            "Disco", "Disco Polo", "Euro-Disco", "Eurobeat", "Eurodance", "Freestyle",
            "Hands Up", "Hi NRG", "Italo-Disco", "Italodance", "Nu-Disco", "New Beat",
        ],
        "synth": [
            "Chiptune", "Darkwave", "EBM", "Electro", "Electroclash", "New Wave",
            "Synth-pop", "Synthwave", "Vaporwave",
        ],
        "ambient": [
            "Ambient", "Berlin-School", "Chillwave", "Dark Ambient", "Downtempo", "Drone",
            "Dub", "Illbient", "New Age", "Trip Hop", "Modern Classical", "Dungeon Synth",
        ],
        "experimental": [
            "Abstract", "Breakcore", "Experimental", "Glitch", "IDM", "Industrial",
            "Leftfield", "Musique Concrète", "Noise", "Power Electronics", "Rhythmic Noise",
            "Sound Collage",
        ],
        "pop": ["Dance-pop"],
        "hip_hop": ["Hip Hop"],
        "jazz": ["Acid Jazz", "Future Jazz", "Jazzdance"],
        "latin": ["Latin"],
        "folk": ["Neofolk"],
    },
    "Folk, World, & Country": {
        "folk": [
            "Bluegrass", "Cajun", "Celtic", "Country", "Folk", "Hillbilly", "Honky Tonk", "Nordic",
        ],
        "soul": ["Gospel"],
    },
    "Funk / Soul": {
        "dance": ["Disco"],
        "world": ["Afrobeat"],
    },
    "Hip Hop": {
        "bass": ["Bass Music", "Grime"],
        "ambient": ["Trip Hop"],
        "soul": ["RnB/Swing"],
    },
    "Jazz": {
        "world": ["Afrobeat"],
        "latin": ["Afro-Cuban Jazz", "Bossa Nova", "Latin Jazz"],
    },
    "Reggae": {
        "world": ["Calypso", "Soca"],
    },
    "Rock": {
        "metal": [
            "Atmospheric Black Metal", "Black Metal", "Death Metal", "Deathcore",
            "Depressive Black Metal", "Doom Metal", "Folk Metal", "Funeral Doom Metal",
            "Funk Metal", "Goregrind", "Gothic Metal", "Grindcore", "Heavy Metal",
            "Melodic Death Metal", "Metalcore", "Nu Metal", "Pornogrind", "Post-Metal",
            "Power Metal", "Progressive Metal", "Sludge Metal", "Speed Metal",
            "Technical Death Metal", "Thrash", "Viking Metal",
        ],
        "punk": [
            "Crust", "Hardcore", "Melodic Hardcore", "Noisecore", "Oi", "Pop Punk",
            "Post-Hardcore", "Power Violence", "Psychobilly", "Punk",
        ],
        "indie": [
            "Alternative Rock", "Brit Pop", "Coldwave", "Dream Pop", "Emo", "Ethereal",
            "Goth Rock", "Grunge", "Indie Rock", "Lo-Fi", "Math Rock", "No Wave",
            "Post Rock", "Post-Punk", "Shoegaze", "Deathrock",
        ],
        "synth": ["New Wave"],
        "experimental": ["Avantgarde", "Experimental", "Industrial", "Noise"],
        "folk": ["Country Rock", "Folk Rock", "Neofolk"],
        "reggae": ["Ska"],
        "pop": ["Power Pop", "Yé-Yé"],
        "jazz": ["Lounge"],
    },
    "Pop": {
        "indie": ["Indie Pop"],
    },
}


def _build_discogs_map() -> Dict[str, str]:
    out: Dict[str, str] = {}
    for parent, by_genre in _DISCOGS_OVERRIDES.items():
        for genre, subs in by_genre.items():
            for sub in subs:
                out[f"{parent}---{sub}"] = genre
    return out


_DISCOGS_MAP = _build_discogs_map()


def discogs_label_genre(label: str) -> str:
    """Station genre for a Discogs400 label such as "Electronic---Techno"."""
    if label in _DISCOGS_MAP:
        return _DISCOGS_MAP[label]
    parent = label.split("---", 1)[0].strip()
    return _DISCOGS_PARENT_DEFAULT.get(parent, "other")


def discogs_distribution(labels: List[str], activations) -> Dict[str, float]:
    """Turn Discogs400 activations into a station-genre distribution.

    Each station genre scores the strongest activation among its labels
    (max, not sum): a genre with many labels must not win by adding up
    many small activations.
    """
    best: Dict[str, float] = {}
    for label, value in zip(labels, activations):
        genre = discogs_label_genre(label)
        v = float(value)
        if v > best.get(genre, 0.0):
            best[genre] = v
    return normalize(best)


# --- Free-text genres (file tags, MusicBrainz) -------------------------------

# Checked in order; the first pattern that matches a tag wins.
_TEXT_RULES = [
    (r"drum ?(n|and|&|'n'|n') ?bass|\bdnb\b|\bd&b\b|jungle|liquid funk|neurofunk", "drum_and_bass"),
    (r"hardstyle|gabber|happy hardcore|hardcore techno|frenchcore|hard dance|jumpstyle|speedcore|uptempo", "hard_dance"),
    (r"techno|schranz|^minimal$", "techno"),
    (r"house|big room", "house"),
    (r"trance|\bpsy\b|goa", "trance"),
    (r"dubstep|breakbeat|\bbreaks\b|uk garage|\bgarage\b(?! rock)|grime|bass music|riddim|future bass|bassline", "bass"),
    (r"disco|eurodance|eurobeat|hi.?nrg|italo", "dance"),
    (r"synth.?pop|synthwave|darkwave|\belectro\b|electroclash|new wave|ebm|vaporwave|retrowave|chiptune", "synth"),
    (r"ambient|chill|downtempo|lo.?fi|new age|trip.?hop|drone", "ambient"),
    (r"experimental|idm|glitch|noise|industrial|avant", "experimental"),
    (r"hip.?hop|\brap\b|trap|grime|drill|boom bap|g-funk", "hip_hop"),
    (r"\br&b\b|\brnb\b|rhythm (and|&) blues|soul|funk|motown|gospel|new jack", "soul"),
    (r"metal|thrash|grindcore|djent", "metal"),
    (r"punk|hardcore|\boi\b|emo\b|screamo", "punk"),
    (r"indie|alternative|\balt\b|grunge|shoegaze|post.?rock|brit.?pop|dream pop|math rock", "indie"),
    (r"rock|rockabilly|surf", "rock"),
    (r"jazz|swing|bebop|bossa", "jazz"),
    (r"blues", "blues"),
    (r"classical|baroque|opera|symphon|orchestr|chamber|romantic era", "classical"),
    (r"country|folk|bluegrass|americana|singer.?songwriter|celtic", "folk"),
    (r"latin|reggaeton|salsa|cumbia|bachata|tango|samba|merengue|flamenco", "latin"),
    (r"reggae|dancehall|\bska\b|\bdub\b|rocksteady", "reggae"),
    (r"afro|world|african|bollywood|arabic|turkish|balkan", "world"),
    (r"soundtrack|score|musical|\bost\b|video game|film", "soundtrack"),
    (r"pop|schlager|chanson|ballad|dance.?pop", "pop"),
    (r"spoken|comedy|audiobook|podcast|children|christmas|holiday", "other"),
]
_COMPILED_RULES = [(re.compile(p, re.IGNORECASE), g) for p, g in _TEXT_RULES]

# Tags that only say "electronic music" spread evenly over the electronic genres.
_GENERIC_ELECTRONIC = re.compile(r"^(electronic|electronica|edm|dance|club|electro.?dance|electronic dance music)$", re.I)

_TAG_SPLIT = re.compile(r"\s*(?:;|,|/|\\|\|| - )\s*")


def split_tag(raw: str) -> List[str]:
    """Split a raw genre tag such as "Techno; Hard Techno" into its parts."""
    return [p for p in (s.strip() for s in _TAG_SPLIT.split(raw or "")) if p]


def text_genre(text: str) -> Optional[Dict[str, float]]:
    """Station-genre distribution for one free-text genre, or None if unknown."""
    t = text.strip()
    if not t:
        return None
    if _GENERIC_ELECTRONIC.match(t):
        w = 1.0 / len(ELECTRONIC_GENRES)
        return {g: w for g in ELECTRONIC_GENRES}
    for pattern, genre in _COMPILED_RULES:
        if pattern.search(t):
            return {genre: 1.0}
    return None


def text_distribution(texts: List[str], weights: Optional[List[float]] = None) -> Dict[str, float]:
    """Blend several free-text genres (optionally weighted, e.g. MusicBrainz vote counts)."""
    total: Dict[str, float] = {}
    for i, text in enumerate(texts):
        dist = text_genre(text)
        if not dist:
            continue
        w = weights[i] if weights else 1.0
        for g, v in dist.items():
            total[g] = total.get(g, 0.0) + w * v
    return normalize(total)


def normalize(scores: Dict[str, float]) -> Dict[str, float]:
    total = sum(v for v in scores.values() if v > 0)
    if total <= 0:
        return {}
    return {g: v / total for g, v in scores.items() if v > 0}
