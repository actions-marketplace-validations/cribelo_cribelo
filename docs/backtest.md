# Measuring it on your own history

`cribelo backtest` replays the first-parent history of the base branch. For each past change it rebuilds the plan from the code of that time, then looks at the ninety days that followed: a file counts as fixed when a commit whose message says fix, revert, regression or the equivalent in a few languages touches it. The output is the share of those files the reading list contained, and the share of the changed lines you would have read to get there, for the top three files, for the rule the plan applies (every high file, at least three), and for reading the largest files first.

```
$ cribelo backtest
79 changes qualified, 2023-10-22 → 2026-07-28: 1007 code files, 151 of them (15%) later touched by a fix.

generic weights, all qualified changes
  Reading the top-3 files covered 40% of the files later fixed, for 59% of the changed lines.
  Reading the 3 largest files instead: 36% of the files later fixed, for 63% of the lines.
  Reading every high file, at least 3 (3.7 files a change): 44% of the files later fixed, for 71% of the lines.
```

These figures are from the AST Metrics repository. A fix commit touching a file later is a noisy label, the fix may be elsewhere, so read the result as a comparison with the naive plan rather than as a verdict.

`cribelo calibrate` fits a logistic regression on the oldest 70 % of those changes, scores it on the most recent 30 %, and writes the weights to `.cribelo/weights.json` only when they beat the defaults on that held-out part, with the top three files and with every high file. The high and medium thresholds move with the weights, so that a calibrated model flags the same share of files as the generic one. Commit the file; every plan then uses it. This is a quarterly job, not a CI step.
