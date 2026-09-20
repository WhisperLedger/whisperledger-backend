import { initializeApp } from "firebase/app";
import { getAuth, signInWithEmailAndPassword } from "firebase/auth";
import { collection, doc, getFirestore, writeBatch } from "firebase/firestore";

const firebaseConfig = {
  apiKey: "AIzaSyDyWPFvO_MDqJbRdMjIv0WUJP4wO93dFnA",
  authDomain: "whisperledger-94715.firebaseapp.com",
  projectId: "whisperledger-94715",
  storageBucket: "whisperledger-94715.firebasestorage.app",
  messagingSenderId: "1064792050841",
  appId: "1:1064792050841:web:9e30d27d9f6aa2e4fe3c08",
};

const email = process.env.IMPORT_EMAIL;
const password = process.env.IMPORT_PASSWORD;

if (!email || !password) {
  throw new Error("Set IMPORT_EMAIL and IMPORT_PASSWORD before running this script.");
}

const expenses = [
  ["2026-04-01", 110, "Lunch", "food_outside", "upi"],
  ["2026-04-01", 80, "Bread+Panner", "groceries", "upi"],
  ["2026-04-01", 110, "Coconut Water", "food_outside", "upi"],
  ["2026-04-01", 132, "Auto", "travel", "upi"],
  ["2026-04-01", 20000, "Ghar", "rent", "upi"],
  ["2026-04-01", 800, "Cook", "utilities", "upi"],
  ["2026-04-01", 6900, "Sayan rent", "rent", "upi"],
  ["2026-04-01", 2000, "card", "other", "card"],
  ["2026-04-02", 140, "Auto", "travel", "upi"],
  ["2026-04-02", 42, "kitchen", "groceries", "upi"],
  ["2026-04-02", 355, "dinner", "food_outside", "upi"],
  ["2026-04-03", 43, "Dal", "groceries", "upi"],
  ["2026-04-03", 89, "bottel", "groceries", "upi"],
  ["2026-04-04", 417, "Maid", "utilities", "upi"],
  ["2026-04-04", 136, "cleaning items", "utilities", "upi"],
  ["2026-04-04", 50, "plumber", "utilities", "upi"],
  ["2026-04-04", 250, "biryani", "food_outside", "upi"],
  ["2026-04-04", 3355, "concert", "entertainment", "upi"],
  ["2026-04-05", 1100, "kitchen+bedsheet", "shopping", "upi"],
  ["2026-04-05", 300, "lunch", "food_outside", "upi"],
  ["2026-04-05", 49000, "invest", "invest", "upi"],
  ["2026-04-05", 7000, "Papa", "transfer", "upi"],
  ["2026-04-06", 162, "travelling", "travel", "upi"],
  ["2026-04-07", 270, "kitchen", "groceries", "upi"],
  ["2026-04-07", 166, "travelling", "travel", "upi"],
  ["2026-04-08", 170, "lunch", "food_outside", "upi"],
  ["2026-04-08", 163, "travelling", "travel", "upi"],
  ["2026-04-08", 220, "kitchen", "groceries", "upi"],
  ["2026-04-08", 128, "kitchen", "groceries", "upi"],
  ["2026-04-09", 160, "Travelling", "travel", "upi"],
  ["2026-04-09", 110, "Snacks", "food_outside", "upi"],
  ["2026-04-10", 165, "Travelling", "travel", "upi"],
  ["2026-04-10", 1000, "cook", "utilities", "upi"],
  ["2026-04-10", 34, "kitchen", "groceries", "upi"],
  ["2026-04-11", 334, "G", "other", "upi"],
  ["2026-04-11", 90, "kitchen", "groceries", "upi"],
  ["2026-04-11", 605, "movie", "entertainment", "upi"],
  ["2026-04-11", 161, "travelling", "travel", "upi"],
  ["2026-04-11", 150, "food", "food_outside", "upi"],
  ["2026-04-12", 428, "travelling", "travel", "upi"],
  ["2026-04-12", 212, "lunch", "food_outside", "upi"],
  ["2026-04-12", 902, "face", "health", "upi"],
  ["2026-04-12", 127, "kitchen", "groceries", "upi"],
  ["2026-04-12", 190, "bodywash", "health", "upi"],
  ["2026-04-12", 129, "kitchen", "groceries", "upi"],
  ["2026-04-13", 204, "travelling", "travel", "upi"],
  ["2026-04-13", 360, "breakfast and lunch", "food_outside", "upi"],
  ["2026-04-13", 173, "kitchen", "groceries", "upi"],
  ["2026-04-13", 500, "light", "utilities", "upi"],
  ["2026-04-14", 910, "electricity", "utilities", "upi"],
  ["2026-04-14", 78, "snacks", "food_outside", "upi"],
  ["2026-04-14", 19000, "card", "other", "card"],
  ["2026-04-15", 162, "travelling", "travel", "upi"],
  ["2026-04-15", 220, "poker", "entertainment", "upi"],
  ["2026-04-15", 112, "kitchen", "groceries", "upi"],
  ["2026-04-16", 95, "kitchen", "groceries", "upi"],
  ["2026-04-16", 156, "travelling", "travel", "upi"],
  ["2026-04-17", 173, "travelling", "travel", "upi"],
  ["2026-04-17", 276, "kitchen", "groceries", "upi"],
  ["2026-04-18", 182, "batball", "entertainment", "upi"],
  ["2026-04-18", 372, "pizza", "food_outside", "upi"],
  ["2026-04-19", 76, "kitchen", "groceries", "upi"],
  ["2026-04-19", 91, "kitchen", "groceries", "upi"],
  ["2026-04-19", 275, "lunch", "food_outside", "upi"],
  ["2026-04-19", 325, "dinner", "food_outside", "upi"],
  ["2026-04-20", 285, "kitchen", "groceries", "upi"],
  ["2026-04-20", 120, "travelling", "travel", "upi"],
  ["2026-04-21", 145, "Lunch", "food_outside", "upi"],
  ["2026-04-21", 130, "travelling", "travel", "upi"],
  ["2026-04-21", 250, "bathroom cleaning", "utilities", "upi"],
  ["2026-04-21", 60, "kitchen oil", "groceries", "upi"],
  ["2026-04-22", 200, "Hair", "health", "upi"],
  ["2026-04-22", 70, "snacks", "food_outside", "upi"],
  ["2026-04-22", 150, "travelling", "travel", "upi"],
  ["2026-04-23", 900, "shower", "shopping", "upi"],
  ["2026-04-23", 60, "kitchen", "groceries", "upi"],
  ["2026-04-23", 100, "kitchen", "groceries", "upi"],
  ["2026-04-23", 155, "Travelling", "travel", "upi"],
  ["2026-04-24", 152, "kitchen", "groceries", "upi"],
  ["2026-04-24", 150, "travelling", "travel", "upi"],
  ["2026-04-25", 125, "plumber", "utilities", "upi"],
  ["2026-04-25", 70, "moov", "health", "upi"],
];

async function main() {
  console.log("Initializing Firebase...");
  const app = initializeApp(firebaseConfig);
  const auth = getAuth(app);
  const db = getFirestore(app);

  console.log(`Signing in as ${email}...`);
  const { user } = await signInWithEmailAndPassword(auth, email, password);
  console.log(`Signed in successfully. UID: ${user.uid}`);

  const expensesRef = collection(db, `users/${user.uid}/expenses`);
  const batch = writeBatch(db);

  // Ensure the parent profile exists. Firestore permits subcollections under a
  // missing parent document, which makes the ledger invisible to dashboards
  // that enumerate the top-level users collection.
  batch.set(doc(db, "users", user.uid), {
    uid: user.uid,
    email: user.email || email,
    displayName: user.displayName || "WhisperLedger User",
    status: "active",
    updatedAt: Date.now(),
  }, { merge: true });

  console.log(`Preparing batch for ${expenses.length} expenses...`);
  for (const [date, amount, note, categoryId, paymentMode] of expenses) {
    const docRef = doc(expensesRef);
    batch.set(docRef, {
      amount,
      note,
      categoryId,
      paymentMode,
      date,
      createdAt: Date.now(),
    });
  }

  console.log("Committing batch to Firestore...");
  await batch.commit();
  console.log(`Successfully imported ${expenses.length} expenses for user ${user.uid}.`);
}

main().catch((error) => {
  console.error("Error during import:");
  console.error(error);
  process.exit(1);
});
