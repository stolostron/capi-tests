#!/usr/bin/env python3
"""Offline regression orchestration tests; no Go phases or cloud CLIs execute."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


class ARORegressionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        (self.work / "test").mkdir()
        (self.work / "test/config.go").write_text('const DefaultCAPIUser = "fixture"\n')
        self.trace = self.work / "trace.jsonl"
        self.fake = self.work / "make"
        self.fake.write_text(f"#!{sys.executable}\n" + '''import json, os, sys
from pathlib import Path
args = sys.argv[1:]
target = next(a for a in args if not a.startswith('-') and '=' not in a)
with open(os.environ['TRACE'], 'a') as stream:
    stream.write(json.dumps({'target': target, 'args': args}) + '\\n')
sys.exit(9 if target == os.environ.get('FAIL_PHASE') else 0)
''')
        self.fake.chmod(0o700)
        self.env = dict(os.environ, TRACE=str(self.trace), INFRA_PROVIDER="aro",
                        ARO_UPGRADE_VERSION="4.99", ARO_UPGRADE_NODE_POOL_VERSION="",
                        RUN_ARO_UPGRADE="false", FAIL_PHASE="")

    def run_make(self, target, **env):
        result = subprocess.run([
            "make", "--no-print-directory", "-f", str(ROOT / "Makefile"), target,
            f"MAKE={self.fake}", "CAPI_USER_DEFAULT=fixture", "CAPI_USER=fixture",
            f"ARO_REPO_DIR={self.work / 'installer'}", f"RESULTS_DIR={self.work / 'results'}",
            f"LATEST_RESULTS_DIR={self.work / 'latest'}", "GOBIN=/nonexistent",
        ], cwd=self.work, env=dict(self.env, **env), capture_output=True, text=True, timeout=10)
        records = [json.loads(line) for line in self.trace.read_text().splitlines()] if self.trace.exists() else []
        return result, records

    def test_opt_in_runs_upgrade_between_verification_and_deletion(self):
        result, records = self.run_make("_test-all-impl", RUN_ARO_UPGRADE="true")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual([r["target"] for r in records], [
            "_init-run-context", "_check-dep", "_setup", "_management_cluster",
            "_generate-yamls", "_deploy-crs", "_verify-workload-cluster",
            "_upgrade-workload-cluster", "_delete-workload-cluster", "_mce-teardown",
        ])
        for record in records[1:]:
            self.assertIn("RUN_CONTEXT_INITIALIZED=1", record["args"])
            self.assertIn(f"RESULTS_DIR={self.work / 'results'}", record["args"])

    def test_default_lifecycle_does_not_upgrade_even_when_target_is_set(self):
        result, records = self.run_make("_test-all-impl")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("_upgrade-workload-cluster", [r["target"] for r in records])
        self.assertIn("_delete-workload-cluster", [r["target"] for r in records])

    def test_failed_upgrade_stops_deletion_and_restores_mce(self):
        result, records = self.run_make("_test-all-impl", RUN_ARO_UPGRADE="true", FAIL_PHASE="_upgrade-workload-cluster")
        self.assertNotEqual(result.returncode, 0)
        targets = [r["target"] for r in records]
        self.assertIn("_upgrade-workload-cluster", targets)
        self.assertNotIn("_delete-workload-cluster", targets)
        self.assertEqual(targets[-1], "_mce-teardown")

    def test_failed_create_does_not_upgrade_or_delete(self):
        result, records = self.run_make("_test-all-impl", RUN_ARO_UPGRADE="true", FAIL_PHASE="_deploy-crs")
        self.assertNotEqual(result.returncode, 0)
        targets = [r["target"] for r in records]
        self.assertNotIn("_upgrade-workload-cluster", targets)
        self.assertNotIn("_delete-workload-cluster", targets)
        self.assertEqual(targets[-1], "_mce-teardown")

    def test_regression_entrypoint_enables_existing_pipeline(self):
        result, records = self.run_make("test-regression")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual([r["target"] for r in records], ["test-all"])
        self.assertIn("RUN_ARO_UPGRADE=true", records[0]["args"])

    def test_invalid_requests_fail_before_starting_any_phase(self):
        for env in ({"ARO_UPGRADE_VERSION": ""}, {"INFRA_PROVIDER": "rosa"},
                    {"ARO_UPGRADE_VERSION": "invalid"}, {"ARO_UPGRADE_NODE_POOL_VERSION": "invalid"}):
            with self.subTest(env=env):
                result, records = self.run_make("test-regression", **env)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(records, [])

    def test_ci_preflight_and_dispatch(self):
        import yaml  # PyYAML is needed only for executing the checked-in workflow snippets.
        workflow = yaml.load((ROOT / ".github/workflows/workload-cluster-aro.yml").read_text(), Loader=yaml.BaseLoader)
        steps = workflow["jobs"]["management-cluster"]["steps"]
        preflight = next(step["run"] for step in steps if step.get("id") == "regression-settings")
        dispatch = next(step["run"] for step in steps if step.get("id") == "phase3")
        for version, pool, deployment, deletion, expected in [
            ("4.99", "", "60", "60", "test-regression"),
            ("", "", "60", "60", "test-all"),
            ("4.99", "", "120", "120", None),
            ("invalid", "", "60", "60", None),
            ("", "4.99", "60", "60", None),
        ]:
            with self.subTest(version=version, pool=pool, deployment=deployment, deletion=deletion):
                if self.trace.exists():
                    self.trace.unlink()
                env = dict(self.env, PATH=f"{self.work}:{os.environ['PATH']}",
                           ARO_UPGRADE_VERSION=version, ARO_UPGRADE_NODE_POOL_VERSION=pool,
                           CLUSTER_DEPLOYMENT_TIMEOUT_MINUTES=deployment,
                           CLUSTER_DELETION_TIMEOUT_MINUTES=deletion)
                result = subprocess.run(["bash", "-e", "-c", preflight + "\n" + dispatch],
                                        env=env, cwd=self.work, capture_output=True, text=True, timeout=10)
                if expected is None:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertFalse(self.trace.exists())
                else:
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    record = json.loads(self.trace.read_text())
                    self.assertEqual(record["target"], expected)

    def test_upgrade_failure_still_copies_junit(self):
        gotestsum = self.work / "gotestsum"
        gotestsum.write_text(f"#!{sys.executable}\n" + '''from pathlib import Path
import sys
path = next(a.split('=', 1)[1] for a in sys.argv if a.startswith('--junitfile='))
Path(path).write_text('<testsuite failures="1"/>')
sys.exit(1)
''')
        gotestsum.chmod(0o700)
        result = subprocess.run([
            "make", "--no-print-directory", "-f", str(ROOT / "Makefile"), "_upgrade-workload-cluster",
            "RUN_CONTEXT_INITIALIZED=1", f"GOTESTSUM={gotestsum}", f"GOBIN={self.work}",
            "CAPI_USER_DEFAULT=fixture", f"ARO_REPO_DIR={self.work / 'installer'}",
            f"RESULTS_DIR={self.work / 'results'}", f"LATEST_RESULTS_DIR={self.work / 'latest'}",
        ], cwd=self.work, env=self.env, capture_output=True, text=True, timeout=10)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.work / "latest/junit-upgrade.xml").read_text(), '<testsuite failures="1"/>')


if __name__ == "__main__":
    unittest.main()
