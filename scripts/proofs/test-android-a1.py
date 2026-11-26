#!/usr/bin/env python3
"""Offline tests for the A1 proof's parsers and fail-closed evidence checks."""
import importlib.util
from pathlib import Path
import unittest

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("android_a1_proof", HERE / "android-a1-account-boundary.py")
proof = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(proof)


class AndroidA1ProofTests(unittest.TestCase):
    def test_assetlinks_document_is_exact(self):
        self.assertEqual(
            proof.assetlinks_document(),
            '[\n  {\n    "relation": [\n      "delegate_permission/common.get_login_creds"\n    ],\n    "target": {\n      "namespace": "android_app",\n      "package_name": "news.choir.autoradio.lab.one",\n      "sha256_cert_fingerprints": [\n        "DF:E9:76:7C:64:2A:2F:86:4B:29:70:49:BD:E9:7C:08:36:CB:58:D5:CE:51:6F:FE:7A:7D:A1:1F:6F:EC:9E:A7"\n      ]\n    }\n  }\n]\n',
        )

    def test_tree_values_find_nested_fact_rows(self):
        tree = '''View#1
  Row#2 [settings-account-signed]
    Text#3 "Yes"
  Row#4 [settings-account-handle]
    Text#5 "writer-abcdef"
  Button#6 [settings-signout]
'''
        self.assertEqual(proof.tree_value(tree, "settings-account-signed"), "Yes")
        self.assertEqual(proof.tree_value(tree, "settings-account-handle"), "writer-abcdef")
        self.assertIn("[settings-signout]", tree)

    def test_apksigner_reader_requires_the_canonical_certificate(self):
        out = '''Signer #1 certificate DN: CN=Android Debug
Signer #1 certificate SHA-256 digest: dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7
Signer #1 certificate SHA-1 digest: 00
'''
        self.assertEqual(proof.apksigner_digest(out), proof.CERT_SHA256)
        with self.assertRaises(proof.ProofError):
            proof.apksigner_digest(out.replace(proof.CERT_SHA256, "0" * 64))

    def test_biometric_action_requires_the_exact_confirmation(self):
        with self.assertRaises(proof.ProofError):
            proof.require_biometric_confirmation(None)
        proof.require_biometric_confirmation(proof.BIOMETRIC_CONFIRMATION)
        with self.assertRaises(proof.ProofError):
            proof.require_biometric_confirmation("yes")

    def test_summary_fails_closed_without_every_receipt(self):
        complete = {
            "status": "pass",
            "commit": "a" * 40,
            "package": proof.PACKAGE,
            "certificate_sha256": proof.CERT_SHA256,
            "apk_sha256": "b" * 64,
            "serial": proof.SERIAL,
            "drill_handle": "writer-abcdef",
            "android_origin": proof.ANDROID_ORIGIN,
            "biometric_owner_action": True,
            "checks": {name: True for name in proof.REQUIRED_CHECKS},
        }
        proof.validate_summary(complete)
        for key in complete:
            bad = dict(complete)
            del bad[key]
            with self.assertRaises(proof.ProofError):
                proof.validate_summary(bad)
        bad = dict(complete, status="failed")
        with self.assertRaises(proof.ProofError):
            proof.validate_summary(bad)


if __name__ == "__main__":
    unittest.main()
