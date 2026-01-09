# Test 7: TestVerification_ControllerLogSummary

**Location:** `test/06_verification_test.go:269-337`

**Purpose:** Summarize and save logs from all controllers (CAPI, CAPZ, ASO).

---

## Controllers Analyzed

| Controller | Namespace | Deployment |
|------------|-----------|------------|
| CAPI | `capi-system` | `capi-controller-manager` |
| CAPZ | `capz-system` | `capz-controller-manager` |
| ASO | `capz-system` | `azureserviceoperator-controller-manager` |

---

## Detailed Flow

```
1. Get management cluster context:
   - context = "kind-${MANAGEMENT_CLUSTER_NAME}"

2. Get log summaries for all controllers:
   - Fetch logs using kubectl
   - Count error lines
   - Count warning lines
   - Extract sample errors

3. Get results directory:
   - resultsDir = GetResultsDir()

4. Save complete logs:
   - Write full logs to files
   - Update summaries with file paths

5. Format and display summary:
   - Print error/warning counts
   - Show sample errors
   - Show log file locations

6. Copy to latest directory:
   - If results/latest exists
   - Copy log files for easy access

7. Log summary:
   - Total errors across controllers
   - Total warnings across controllers
   - Log file locations
```

---

## Log Analysis

| Metric | Detection |
|--------|-----------|
| Errors | Lines containing `"error"` or `level=error` |
| Warnings | Lines containing `"warn"` or `level=warn` |

---

## Output Files

| File | Description |
|------|-------------|
| `results/<timestamp>/capi-controller.log` | Full CAPI controller logs |
| `results/<timestamp>/capz-controller.log` | Full CAPZ controller logs |
| `results/<timestamp>/aso-controller.log` | Full ASO controller logs |
| `results/latest/*.log` | Copies for easy access |

---

## Example Output

### Healthy Controllers
```
=== RUN   TestVerification_ControllerLogSummary

===================================================
            CONTROLLER LOG SUMMARY
===================================================

 Controller         | Errors | Warnings | Log File
---------------------------------------------------
 CAPI Controller    |      0 |        2 | results/2024-01-15_14-30-00/capi-controller.log
 CAPZ Controller    |      0 |        5 | results/2024-01-15_14-30-00/capz-controller.log
 ASO Controller     |      0 |        3 | results/2024-01-15_14-30-00/aso-controller.log
---------------------------------------------------
 TOTAL              |      0 |       10 |
---------------------------------------------------

===================================================

    06_verification_test.go:331: Found 10 warnings (no errors) across all controllers.
    06_verification_test.go:335: Controller logs saved to: results/2024-01-15_14-30-00
--- PASS: TestVerification_ControllerLogSummary (1.20s)
```

### Controllers with Errors
```
=== RUN   TestVerification_ControllerLogSummary

===================================================
            CONTROLLER LOG SUMMARY
===================================================

 Controller         | Errors | Warnings | Log File
---------------------------------------------------
 CAPI Controller    |      0 |        2 | results/2024-01-15_14-30-00/capi-controller.log
 CAPZ Controller    |      3 |        8 | results/2024-01-15_14-30-00/capz-controller.log
 ASO Controller     |      1 |        5 | results/2024-01-15_14-30-00/aso-controller.log
---------------------------------------------------
 TOTAL              |      4 |       15 |
---------------------------------------------------

Sample errors from CAPZ Controller:
  - "error reconciling AzureMachine: failed to create VM"
  - "error getting Azure credentials"
  - "error updating status"

Sample errors from ASO Controller:
  - "error reconciling ResourceGroup: subscription not found"

===================================================

    06_verification_test.go:328: Warning: Found 4 errors across all controllers. Review logs for details.
    06_verification_test.go:335: Controller logs saved to: results/2024-01-15_14-30-00
--- PASS: TestVerification_ControllerLogSummary (1.20s)
```

---

## Why This Matters

1. **Debugging** - Quickly identify controller issues
2. **Post-mortem** - Logs preserved for later analysis
3. **CI/CD** - Artifacts for failed runs
4. **Visibility** - Summary without reading full logs

---

## Results Directory Structure

```
results/
├── 2024-01-15_14-30-00/
│   ├── capi-controller.log
│   ├── capz-controller.log
│   └── aso-controller.log
└── latest/           # Symlink or copy to most recent
    ├── capi-controller.log
    ├── capz-controller.log
    └── aso-controller.log
```

---

## Related Helpers

See `test/helpers.go` for:
- `GetAllControllerLogSummaries()` - Fetches and analyzes logs
- `SaveAllControllerLogs()` - Saves logs to files
- `FormatControllerLogSummaries()` - Formats summary table
- `GetResultsDir()` - Gets timestamped results directory

---

## Notes

- This test does not fail even if errors are found in logs
- Designed as an informational/diagnostic test
- Runs at the very end of verification phase
- Log files are preserved even if test suite fails
