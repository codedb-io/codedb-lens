# How the grade is calculated

The grade, the coverage and each finding's confidence are separate things. The report always shows all three.

## Rule outcomes

Each rule ends in one of four states:

| State | Meaning | Effect on the grade |
|---|---|---|
| **OK** | The rule ran and found nothing | Full credit |
| **Finding** | The rule ran and returned rows | No credit (*Confirmed*) or half credit (*Hint*) |
| **No data** | Missing permission, timeout, error or unsupported feature; the reason is in the report | Left out of the grade, lowers the coverage |
| **Not applicable** | The rule doesn't apply to this version or platform (e.g. a SQL Agent rule on Azure SQL Database) | Left out of both (see *No grade* below) |

*No data* is never counted as OK. A rule that couldn't run can't improve the grade.

## Grade

Every rule has a severity, and each severity has a weight:

| Severity | Weight |
|---|---|
| Critical | 10 |
| High | 5 |
| Medium | 2 |
| Low | 1 |

Over the rules that ran (OK or Finding):

```
grade = 100 × Σ (weight × credit) / Σ weight
credit = 1 for OK, 0.5 for a Hint finding, 0 for a Confirmed finding
```

| Letter | Grade |
|---|---|
| A | 90 or more |
| B | 75 or more |
| C | 60 or more |
| D | 40 or more |
| F | below 40 |

If no rule could run, the grade is **?**. It is also **?** (*No grade*) when fewer than 6 rules
apply to the platform: a grade over two or three checks would look better than it is. The findings still show.

## Coverage

```
coverage = rules that ran / rules that apply
```

Below 80%, the report shows a **partial analysis** seal next to the grade: an A with 50% coverage only says that half
of the checks passed. Grant the [minimum permissions](permissions.md) to raise it.

## Confidence

- **Confirmed:** the collected values prove the problem (e.g. no full backup in 9 days).
- **Hint:** the values point to a likely problem that depends on context Lens can't see (e.g. settings left at
  defaults). A hint costs half the weight.

## Reproducibility

Weights, credits and grade limits live in [`rules/scoring.yaml`](https://github.com/codedb-io/codedb-lens/blob/main/rules/scoring.yaml),
not in code. Each report records the Lens version and a **rule catalog** fingerprint (`catalog_version` in the JSON):
the same data with the same catalog always gives the same grade. `compare` warns when two runs used different catalogs.
