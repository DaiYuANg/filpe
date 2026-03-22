"""Artifact output handling."""

from filpe.artifacts.storage import write_artifact


def collect_artifacts_from_result(
    result: dict,
    output_dir: str,
) -> list[dict]:
    """
    If processor returns artifacts in result, write them and return metadata.
    Result may have key 'artifacts' with list of {name, content_base64, media_type}.
    """
    artifacts = result.get("artifacts", [])
    if not artifacts:
        return []
    meta_list: list[dict] = []
    for a in artifacts:
        meta = write_artifact(
            output_dir=output_dir,
            name=a.get("name", "artifact"),
            content_base64=a.get("content_base64", ""),
            media_type=a.get("media_type", "application/octet-stream"),
        )
        meta_list.append(meta)
    return meta_list
