The Inter TTF files here are Latin subsets (ASCII, Latin-1, Latin Extended,
dashes, quotes, arrows). Full Inter is ~2900 glyphs and expands to tens of
megabytes when Fyne/go-text parses GPOS/glyf; the subset is ~600 glyphs and
no layout tables.

To regenerate from a full Inter desktop TTF:

```
python3 -m fontTools.subset Inter-Regular.full.ttf \
  --output-file=Inter-Regular.ttf \
  --unicodes=U+0020-007E,U+00A0-00FF,U+0100-024F,U+2010-2027,U+2018-201F,U+2022,U+2026,U+2030,U+2039-203A,U+20AC,U+2190-2193,U+2212,U+25CF \
  --layout-features= \
  --drop-tables+=GPOS,GSUB,GDEF \
  --no-hinting --desubroutinize \
  --name-IDs='*' --name-legacy --name-languages='*'
```

Same for Inter-SemiBold. See Inter-LICENSE.txt for the SIL OFL.
