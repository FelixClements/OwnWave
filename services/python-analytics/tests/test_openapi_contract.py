"""The Go API proxies some routes to this service unchanged (analytics.Routes).

openapi.yaml marks each with x-python-path. This test fails when a marked
response there no longer matches FastAPI's own schema, so the two can't drift.
"""

import re
from pathlib import Path

import pytest
import yaml

OPENAPI = Path(__file__).resolve().parents[2] / "go-api-server" / "openapi.yaml"
METHODS = {"get", "post", "put", "patch", "delete"}


def _deref(schema, spec):
    while "$ref" in schema:
        node = spec
        for part in schema["$ref"].removeprefix("#/").split("/"):
            node = node[part]
        schema = node
    return schema


def shape(schema, spec):
    """The parts of a JSON schema a client depends on: types, fields, required.

    Titles, descriptions and formats are dropped, and OpenAPI 3.0/3.1 spellings
    of "nullable" are made the same.
    """
    schema = _deref(schema, spec)
    variants = schema.get("anyOf") or schema.get("oneOf")
    if variants:
        kinds = [shape(v, spec) for v in variants if _deref(v, spec).get("type") != "null"]
        out = kinds[0] if len(kinds) == 1 else {"anyOf": kinds}
        return {**out, "nullable": True} if len(kinds) < len(variants) else out
    kind = schema.get("type")
    if isinstance(kind, list):
        rest = [k for k in kind if k != "null"]
        out = shape({**schema, "type": rest[0] if len(rest) == 1 else rest}, spec)
        return {**out, "nullable": True} if len(rest) < len(kind) else out
    nullable = {"nullable": True} if schema.get("nullable") else {}
    if kind == "object" or "properties" in schema:
        out = {"type": "object"}
        if schema.get("properties"):
            out["properties"] = {k: shape(v, spec) for k, v in schema["properties"].items()}
            out["required"] = sorted(schema.get("required", []))
        extra = schema.get("additionalProperties")
        if isinstance(extra, dict) and extra:
            out["additionalProperties"] = shape(extra, spec)
        return {**out, **nullable}
    if kind == "array":
        return {"type": "array", "items": shape(schema.get("items", {}), spec), **nullable}
    return {"type": kind, **nullable} if kind else {}


def ok_schema(op, spec):
    response = _deref(op["responses"]["200"], spec)
    return response["content"]["application/json"]["schema"]


def _template(path):
    return re.sub(r"\{[^}]+\}", "{}", path)


def _proxied():
    spec = yaml.safe_load(OPENAPI.read_text())
    for public, item in spec["paths"].items():
        for method, op in item.items():
            if method in METHODS and "x-python-path" in op:
                yield pytest.param(spec, method, op, id=f"{method.upper()} {public}")


PROXIED = list(_proxied())


def test_openapi_marks_proxied_routes():
    # Guards against the marker being renamed and every case silently vanishing.
    assert len(PROXIED) >= 9


@pytest.mark.parametrize("spec,method,op", PROXIED)
def test_proxied_response_matches_fastapi(spec, method, op):
    import api

    fastapi_spec = api.app.openapi()
    paths = {_template(p): item for p, item in fastapi_spec["paths"].items()}
    python_path = _template(op["x-python-path"])
    assert python_path in paths, f"FastAPI has no route {op['x-python-path']}"
    assert method in paths[python_path], f"FastAPI has no {method.upper()} {op['x-python-path']}"

    want = shape(ok_schema(op, spec), spec)
    got = shape(ok_schema(paths[python_path][method], fastapi_spec), fastapi_spec)
    assert got == want, f"openapi.yaml and FastAPI disagree on {method.upper()} {op['x-python-path']}"


def test_scan_body_is_optional():
    # SoulLite and the admin UI may post no body; Python then scans MUSIC_DIR.
    import api

    body = api.app.openapi()["paths"]["/scan"]["post"].get("requestBody", {})
    assert not body.get("required", False)
