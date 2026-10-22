# codeDB Lens

**See what your database isn't showing you.** A free, read-only health check for SQL Server and PostgreSQL.

Lens runs a catalog of evidence-based rules and writes a self-contained HTML + JSON report with an
A–F grade and a coverage seal.

## How to read the report

- **Grade (A–F):** weighted by severity (Critical 10, High 5, Medium 2, Low 1) over the rules that ran.
  A ≥ 90, B ≥ 75, C ≥ 60, D ≥ 40, F < 40.
- **Coverage:** rules that ran ÷ rules that apply. Below 80%, the grade shows a *partial analysis* seal with
  what was missing and the permission that would fix it.
- **Confidence:** *Confirmed* (the evidence proves it), *Hint* (likely, needs a human look) or *No data*
  (the rule couldn't run, and the reason is shown).

The full formula is in [How the grade is calculated](methodology.md). Want to see it before installing? Open the
[sample report](demo/report.html) (synthetic data).

Lens shows a snapshot. For history, baselines and alerts, see
[codeDB](https://codedb.com.br/?utm_source=lens&utm_medium=docs&utm_campaign=home).
