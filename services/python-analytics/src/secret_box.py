"""Encrypt the outside source secrets (Discogs token, Last.fm key) at rest.

The admin "Outside sources" page saves them in the database, encrypted with
AES-256-GCM. The key never touches the database: it is derived (HKDF-SHA256)
from SECRETS_KEY, or from ANALYTICS_API_SECRET when SECRETS_KEY is unset, so
a database dump or backup alone does not reveal a saved secret.

Each ciphertext is bound to its source name (GCM associated data), so a
token stored for one source cannot be swapped onto another. When the key
changes (SECRETS_KEY set later, or ANALYTICS_API_SECRET rotated), saved
secrets can no longer be read: the page then says so, the env value is used,
and the admin saves the secret again.
"""

import os
from typing import Optional

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives.kdf.hkdf import HKDF

VERSION = b"\x01"
NONCE_LEN = 12
_INFO = b"ownwave outside source secrets v1"


class SecretUnreadable(Exception):
    """The stored secret was encrypted with another key, or is damaged."""


def key_material() -> str:
    """The env value the encryption key is derived from ("" when none is set)."""
    import config

    return (config.SECRETS_KEY or config.ANALYTICS_API_SECRET or "").strip()


def _key(material: Optional[str] = None) -> bytes:
    material = key_material() if material is None else material
    if not material:
        raise RuntimeError("set SECRETS_KEY or ANALYTICS_API_SECRET to save secrets")
    return HKDF(algorithm=hashes.SHA256(), length=32, salt=None, info=_INFO).derive(material.encode("utf-8"))


def encrypt(plaintext: str, context: str, material: Optional[str] = None) -> bytes:
    """Version byte, random nonce, then the ciphertext with its tag."""
    nonce = os.urandom(NONCE_LEN)
    sealed = AESGCM(_key(material)).encrypt(nonce, plaintext.encode("utf-8"), context.encode("utf-8"))
    return VERSION + nonce + sealed


def decrypt(blob: bytes, context: str, material: Optional[str] = None) -> str:
    blob = bytes(blob)
    if len(blob) <= 1 + NONCE_LEN or blob[:1] != VERSION:
        raise SecretUnreadable("unknown secret format")
    nonce, sealed = blob[1 : 1 + NONCE_LEN], blob[1 + NONCE_LEN :]
    try:
        plain = AESGCM(_key(material)).decrypt(nonce, sealed, context.encode("utf-8"))
    except InvalidTag:
        raise SecretUnreadable("the secret was saved with another encryption key") from None
    return plain.decode("utf-8")


def last4(secret: str) -> str:
    """The most of a secret the API shows; nothing for a secret this short."""
    return secret[-4:] if len(secret) >= 12 else ""
