"""Image processor implementations."""

import base64
from io import BytesIO
from typing import Any

from PIL import Image

from filpe.models.job import StagedInput


def _image_to_artifact(img: Image.Image, fmt: str, quality: int | None, **save_kwargs: Any) -> dict:
    """Encode image to base64 artifact."""
    buf = BytesIO()
    save_opts: dict[str, Any] = {"format": fmt.upper()}
    if quality is not None and fmt.upper() in ("JPEG", "WEBP"):
        save_opts["quality"] = min(95, max(1, quality))
    save_opts.update(save_kwargs)
    img.save(buf, **save_opts)
    buf.seek(0)
    content_b64 = base64.b64encode(buf.read()).decode("ascii")
    mime = {
        "JPEG": "image/jpeg",
        "PNG": "image/png",
        "WEBP": "image/webp",
        "GIF": "image/gif",
    }.get(fmt.upper(), "image/png")
    ext = fmt.lower() if fmt.lower() != "jpeg" else "jpg"
    return {
        "name": f"output.{ext}",
        "content_base64": content_b64,
        "media_type": mime,
    }


class ImageResizeProcessor:
    """Processor: image.resize - proportional scaling (maintain aspect ratio)."""

    name = "image.resize"

    def run(self, staged: StagedInput, options: dict[str, Any] | None) -> dict[str, Any]:
        """
        Resize image proportionally.
        Options:
          - max_width: max width in pixels (scale to fit)
          - max_height: max height in pixels (scale to fit)
          - scale: scale factor (e.g. 0.5 = 50%). Applied if max_width/max_height not set.
          - format: output format (jpeg, png, webp). Default: same as input.
          - quality: JPEG/WebP quality 1-95 (default: 85)
        """
        opts = options or {}
        max_width = opts.get("max_width")
        max_height = opts.get("max_height")
        scale = opts.get("scale")
        out_fmt = (opts.get("format") or "png").upper()
        quality = opts.get("quality", 85)

        img = Image.open(staged.path).convert("RGB" if out_fmt in ("JPEG", "WEBP") else "RGBA")
        w, h = img.size

        if max_width is not None or max_height is not None:
            ratio = 1.0
            if max_width is not None and w > max_width:
                ratio = min(ratio, max_width / w)
            if max_height is not None and h > max_height:
                ratio = min(ratio, max_height / h)
            nw, nh = int(w * ratio), int(h * ratio)
        elif scale is not None:
            nw, nh = int(w * scale), int(h * scale)
        else:
            raise ValueError("Provide max_width, max_height, or scale")

        nw, nh = max(1, nw), max(1, nh)
        img = img.resize((nw, nh), Image.Resampling.LANCZOS)

        return {
            "result": {"width": nw, "height": nh, "original_width": w, "original_height": h},
            "artifacts": [_image_to_artifact(img, out_fmt, quality)],
        }


class ImageCropProcessor:
    """Processor: image.crop - crop to specified region."""

    name = "image.crop"

    def run(self, staged: StagedInput, options: dict[str, Any] | None) -> dict[str, Any]:
        """
        Crop image to region.
        Options:
          - left: left edge (px, default 0)
          - top: top edge (px, default 0)
          - width: crop width (px). Required with height.
          - height: crop height (px). Required with width.
          - right: right edge (px). Alternative to left+width.
          - bottom: bottom edge (px). Alternative to top+height.
          - format: output format (jpeg, png, webp). Default: same as input.
          - quality: JPEG/WebP quality 1-95
        """
        opts = options or {}
        left = opts.get("left", 0)
        top = opts.get("top", 0)
        width = opts.get("width")
        height = opts.get("height")
        right = opts.get("right")
        bottom = opts.get("bottom")
        out_fmt = (opts.get("format") or "png").upper()
        quality = opts.get("quality", 85)

        img = Image.open(staged.path).convert("RGB" if out_fmt in ("JPEG", "WEBP") else "RGBA")
        w, h = img.size

        if right is not None and bottom is not None:
            box = (left, top, right, bottom)
        elif width is not None and height is not None:
            box = (left, top, left + width, top + height)
        else:
            raise ValueError("Provide (width, height) or (right, bottom)")

        img = img.crop(box)
        cw, ch = img.size

        return {
            "result": {"width": cw, "height": ch, "region": list(box)},
            "artifacts": [_image_to_artifact(img, out_fmt, quality)],
        }


class ImageCompressProcessor:
    """Processor: image.compress - compress/optimize image."""

    name = "image.compress"

    def run(self, staged: StagedInput, options: dict[str, Any] | None) -> dict[str, Any]:
        """
        Compress image by reducing quality and optionally dimensions.
        Options:
          - quality: JPEG/WebP quality 1-95 (default: 80)
          - max_width: resize to fit before compress
          - max_height: resize to fit before compress
          - format: output format (jpeg, png, webp). Default: jpeg for photos.
          - optimize: enable PNG optimize (default: True)
        """
        opts = options or {}
        quality = opts.get("quality", 80)
        max_width = opts.get("max_width")
        max_height = opts.get("max_height")
        out_fmt = (opts.get("format") or "jpeg").upper()
        optimize = opts.get("optimize", True)

        img = Image.open(staged.path).convert("RGB" if out_fmt in ("JPEG", "WEBP") else "RGBA")
        orig_w, orig_h = img.size

        if max_width is not None or max_height is not None:
            w, h = img.size
            ratio = 1.0
            if max_width is not None and w > max_width:
                ratio = min(ratio, max_width / w)
            if max_height is not None and h > max_height:
                ratio = min(ratio, max_height / h)
            nw, nh = max(1, int(w * ratio)), max(1, int(h * ratio))
            img = img.resize((nw, nh), Image.Resampling.LANCZOS)
        else:
            nw, nh = orig_w, orig_h

        save_kwargs: dict[str, Any] = {}
        if out_fmt == "PNG" and optimize:
            save_kwargs["optimize"] = True

        art = _image_to_artifact(img, out_fmt, quality, **save_kwargs)
        art["name"] = f"compressed.{art['name'].split('.')[-1]}"

        return {
            "result": {
                "width": nw,
                "height": nh,
                "original_width": orig_w,
                "original_height": orig_h,
                "format": out_fmt,
                "quality": quality,
            },
            "artifacts": [art],
        }
