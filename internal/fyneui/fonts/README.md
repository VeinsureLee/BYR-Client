# Embedded font

This prototype embeds the **unmodified Noto Sans SC Regular** font from the
upstream Noto CJK project's Simplified Chinese subset. The complete SC subset is
used rather than a demo-text-only subset, so arbitrary Chinese forum text can
render offline on Windows and Android.

- Source: https://github.com/notofonts/noto-cjk/blob/main/Sans/SubsetOTF/SC/NotoSansSC-Regular.otf
- Upstream Git blob: `fc0fda9394367c16c1af7555a2ae4d5e2e6a6f02`
- SHA-256: `faa6c9df652116dde789d351359f3d7e5d2285a2b2a1f04a2d7244df706d5ea9`
- Size: 8,331,336 bytes.
- License: SIL Open Font License 1.1, reproduced in `LICENSE` and embedded in the
  executable. The font's Adobe copyright notice is reproduced in `LICENSE`
  and remains in the unmodified font metadata.

The prototype uses this one font weight for all text styles. A production client
can provide additional weights after assessing package size and text layout.
