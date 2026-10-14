#!/usr/bin/env python3
"""
WhisperLedger Interactive Database Shell (CLI)
Allows you to directly query, inspect, and update the live database.

Usage:
  ./scripts/db_shell.py
"""

import sys
import json
import subprocess
import urllib.request
import urllib.error

PROJECT_ID = "whisperledger-94715"
DATABASE_URL = f"https://firestore.googleapis.com/v1/projects/{PROJECT_ID}/databases/(default)/documents"

def get_token():
    try:
        res = subprocess.run(["/Users/tanmayagarwal/google-cloud-sdk/bin/gcloud", "auth", "print-access-token"], capture_output=True, text=True, check=True)
        return res.stdout.strip()
    except Exception as e:
        print(f"❌ Error getting gcloud token: {e}")
        sys.exit(1)

def request_api(path, method="GET", data=None):
    token = get_token()
    url = f"{DATABASE_URL}/{path.lstrip('/')}"
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    body = json.dumps(data).encode("utf-8") if data else None
    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as resp:
            return json.loads(resp.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        return {"error": e.code, "message": e.read().decode("utf-8")}
    except Exception as e:
        return {"error": str(e)}

def parse_val(val_obj):
    if not isinstance(val_obj, dict): return val_obj
    for k in ["stringValue", "booleanValue"]:
        if k in val_obj: return val_obj[k]
    if "integerValue" in val_obj: return int(val_obj["integerValue"])
    if "doubleValue" in val_obj: return float(val_obj["doubleValue"])
    if "timestampValue" in val_obj: return val_obj["timestampValue"]
    if "mapValue" in val_obj:
        return {k: parse_val(v) for k, v in val_obj["mapValue"].get("fields", {}).items()}
    if "arrayValue" in val_obj:
        return [parse_val(v) for v in val_obj["arrayValue"].get("values", [])]
    return val_obj

def parse_doc(d):
    data = {}
    for k, v in d.get("fields", {}).items():
        data[k] = parse_val(v)
    data["_id"] = d.get("name", "").split("/")[-1]
    return data

def main():
    print("=================================================================")
    print(f"⚡ WhisperLedger Interactive DB Shell (Project: {PROJECT_ID})")
    print("   Type 'help' for commands or 'exit' / 'quit' to close.")
    print("=================================================================\n")

    while True:
        try:
            cmd = input("whisperledger-db> ").strip()
        except (KeyboardInterrupt, EOFError):
            print("\nBye!")
            break

        if not cmd:
            continue
        if cmd in ["exit", "quit", "q"]:
            print("Bye!")
            break
        elif cmd == "help":
            print("""
Available DB commands:
  users                                 List all users
  user <email_or_uid>                   Show specific user & their recent expenses
  households                            List all households & members
  expenses [limit]                      List all transactions (e.g., 'expenses 10')
  get <collection/doc_id>               Fetch raw Firestore document
  stats                                 Database statistics & totals
  clear                                 Clear terminal screen
  exit / quit                           Exit DB shell
""")
        elif cmd == "clear":
            print("\033c", end="")
        elif cmd == "stats":
            u = request_api("users").get("documents", [])
            h = request_api("households").get("documents", [])
            print(f"📊 Stats: {len(u)} Users | {len(h)} Households")
        elif cmd == "users":
            res = request_api("users")
            docs = [parse_doc(d) for d in res.get("documents", [])]
            print(f"\n{'UID':<30} | {'NAME':<20} | {'EMAIL':<30} | {'PLAN':<8}")
            print("-" * 95)
            for d in docs:
                name = (d.get("displayName") or d.get("name") or "Unnamed")[:18]
                email = str(d.get("email", "No Email"))[:28]
                plan = (d.get("plan") or ("pro" if d.get("isPro") else "free"))[:6]
                print(f"{d['_id']:<30} | {name:<20} | {email:<30} | {plan:<8}")
            print()
        elif cmd.startswith("user "):
            query = cmd.split(" ", 1)[1].strip().lower()
            res = request_api("users")
            docs = [parse_doc(d) for d in res.get("documents", [])]
            matched = [d for d in docs if query in d["_id"].lower() or query in str(d.get("email", "")).lower() or query in str(d.get("displayName", "")).lower()]
            if not matched:
                print(f"No user found matching '{query}'.")
            else:
                for u in matched:
                    print(json.dumps(u, indent=2))
                    # Check expenses
                    exp_res = request_api(f"users/{u['_id']}/expenses")
                    exps = [parse_doc(e) for e in exp_res.get("documents", [])]
                    print(f"Logged expenses ({len(exps)} total):")
                    for e in exps[:5]:
                        print(f"  • ₹{e.get('amount')} at {e.get('merchant', 'Unknown')} [{e.get('category', 'other')}] on {e.get('date', 'N/A')}")
                    if len(exps) > 5:
                        print(f"  ... and {len(exps) - 5} more.")
                    print()
        elif cmd == "households":
            res = request_api("households")
            docs = [parse_doc(d) for d in res.get("documents", [])]
            print(f"\n{'ID':<15} | {'NAME':<25} | {'INVITE CODE':<15}")
            print("-" * 60)
            for d in docs:
                print(f"{d['_id']:<15} | {str(d.get('name', 'Unnamed')):<25} | {str(d.get('inviteCode') or d.get('code') or 'N/A'):<15}")
            print()
        elif cmd.startswith("expenses"):
            parts = cmd.split()
            lim = int(parts[1]) if len(parts) > 1 and parts[1].isdigit() else 15
            res = request_api("users")
            docs = [parse_doc(d) for d in res.get("documents", [])]
            all_exp = []
            for u in docs:
                e_res = request_api(f"users/{u['_id']}/expenses")
                for e in e_res.get("documents", []):
                    parsed = parse_doc(e)
                    parsed["_user"] = u.get("displayName") or u.get("name") or "User"
                    all_exp.append(parsed)
            all_exp.sort(key=lambda x: str(x.get("date", "")), reverse=True)
            print(f"\n{'DATE':<12} | {'AMOUNT':<10} | {'CATEGORY':<14} | {'MERCHANT / NOTE':<25} | {'USER'}")
            print("-" * 95)
            for e in all_exp[:lim]:
                d = str(e.get("date", "N/A"))[:10]
                amt = f"₹{float(e.get('amount', 0)):,.2f}"
                cat = str(e.get("category", "other"))[:13]
                desc = str(e.get("merchant") or e.get("note") or "Unknown")[:24]
                u = e.get("_user", "")[:18]
                print(f"{d:<12} | {amt:<10} | {cat:<14} | {desc:<25} | {u}")
            print(f"\nShowing {min(lim, len(all_exp))} of {len(all_exp)} total expenses.\n")
        elif cmd.startswith("get "):
            path = cmd.split(" ", 1)[1].strip()
            data = request_api(path)
            print(json.dumps(data, indent=2))
        else:
            print(f"Unknown command: '{cmd}'. Type 'help' to see available commands.")

if __name__ == "__main__":
    main()
