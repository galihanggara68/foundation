# Security Policy

The Foundation team takes security, determinism, and software supply-chain integrity seriously. As a code generator and backend compiler, Foundation is designed to produce secure, verifiable code while maintaining strict defense-in-depth on the local host machine.

---

## 🛡️ Supported Versions

We actively maintain and provide security patches for the latest release and the `main` branch:

| Version | Supported          |
| :------ | :----------------- |
| `main`  | :white_check_mark: |
| Latest  | :white_check_mark: |
| < 0.1.0 | :x:                |

---

## 🔒 Security Commitments & Scope

Foundation is built under strict defensive guarantees:

1. **Host Sandboxing & Path Confinement**:
   The compiler and generator manifest emitters must never write or delete files outside the configured `outDir`. Attempts to escape the output boundary via path traversals (`../`), absolute paths, or poisoned manifest entries are treated as critical security bugs.
2. **Subprocess Isolation**:
   External generator plugins (`foundation-gen-*`) must be executed with strict timeouts, process group isolation, and bounded output streams to prevent denial-of-service (output bombs) or hung processes.
3. **AST & Parser Defense**:
   The blueprint parser must handle adversarial syntax inputs, cyclic dependencies, and deeply nested structures without crashing, hanging, or exhausting memory.
4. **Generated Code Safety**:
   Generated backend code (SQL migrations, HTTP routers, transaction managers) must use parameterized queries and safe escaping to prevent SQL injection or unvalidated parameter tampering.

---

## 🚨 Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability or suspect an attack vector:

1. **Submit via GitHub Private Vulnerability Reporting**:
   Navigate to the **Security** tab of the repository on GitHub and select **"Report a vulnerability"**.
2. **Include Technical Details**:
   - A description of the vulnerability and potential impact.
   - Minimal reproduction blueprint (`.fdn`), script, or command line reproducing the issue.
   - Proposed remediations or patches (if available).

### Response Process

* **Acknowledgment**: We acknowledge security reports within 48 hours.
* **Assessment & Fix**: We will triage the severity, develop a fix on a private branch, and run our security suite (`make test-security`) to verify regression safety.
* **Coordinated Disclosure**: Once patched and released, we will credit the reporter and publish an advisory detailing the issue and fix.

Thank you for helping keep the Foundation ecosystem safe and dependable!
