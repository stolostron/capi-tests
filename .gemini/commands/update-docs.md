Update all documentation files to stay in sync with code changes in the ARO-CAPZ test repository.

## Instructions

1. **Detect what changed**: Ask me what changes were made, or analyze recent commits/diffs to identify:
   - New test phases
   - Modified configuration variables
   - New helper functions
   - Changed workflow patterns
   - Updated Makefile targets

2. **Identify affected documentation**:
   - `README.md` - High-level overview, quick start, main workflow
   - `CLAUDE.md` - Development guidelines, patterns, architecture
   - `test/README.md` - Detailed test suite documentation
   - `TEST_COVERAGE.md` - Test coverage analysis (if test coverage changed)
   - `docs/INTEGRATION.md` - Integration patterns (if integration changed)

3. **For each documentation file, check and update**:

### README.md
- Test execution commands match Makefile
- Environment variables are current
- Quick start reflects current workflow
- Prerequisites list is accurate
- Repository structure is current

### CLAUDE.md
- Test Architecture section lists all test phases in order
- Configuration variables are complete
- Helper functions are documented
- Common tasks reflect current Makefile targets
- Examples use current patterns
- Git workflow information is accurate

### test/README.md
- All test phases documented
- Test execution examples are correct
- Configuration section is complete
- Dependencies between phases are clear
- Troubleshooting tips are relevant

### TEST_COVERAGE.md
- Coverage metrics are current (if changed)
- Test phase descriptions match actual implementation
- Gaps/improvements section reflects reality

### docs/INTEGRATION.md
- Integration approaches are current
- Repository URLs and branches are correct
- Examples work with current code

4. **Update each file**:
   - Make precise, focused changes
   - Maintain existing formatting and style
   - Preserve useful examples
   - Keep language concise and technical
   - Use actual file paths and line numbers where relevant

5. **Validation**:
   - Verify all cross-references are valid
   - Check that examples actually work
   - Ensure consistency across all docs

## Specific Checks

**When a new test phase is added**:
- Add to README.md test execution section
- Add to CLAUDE.md test architecture
- Add to test/README.md with full details
- Add to Makefile and document the new target

**When configuration changes**:
- Update all env var lists in README.md
- Update CLAUDE.md configuration section
- Update test/README.md if it affects test behavior
- Verify examples use correct variable names

**When helper functions change**:
- Update CLAUDE.md helper functions section
- Update test/README.md if patterns change
- Ensure examples reflect new signatures

**When workflow changes**:
- Update README.md workflow description
- Update CLAUDE.md development commands
- Update test/README.md test execution flow

## Output

Provide a summary of:
- **Files Updated**: List of documentation files changed
- **Changes Made**: Brief description of each update
- **Validation**: Confirmation that cross-references and examples work
