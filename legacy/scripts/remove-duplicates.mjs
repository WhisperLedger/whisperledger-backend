import { initializeApp } from "firebase/app";
import { getAuth, signInWithEmailAndPassword } from "firebase/auth";
import { collection, getDocs, getFirestore, deleteDoc, doc, query } from "firebase/firestore";

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

async function main() {
  const app = initializeApp(firebaseConfig);
  const auth = getAuth(app);
  const db = getFirestore(app);

  const { user } = await signInWithEmailAndPassword(auth, email, password);
  const expensesRef = collection(db, `users/${user.uid}/expenses`);
  
  console.log("Fetching expenses to remove duplicates...");
  const snapshot = await getDocs(expensesRef);
  
  const seen = new Set();
  let deleted = 0;

  for (const expenseDoc of snapshot.docs) {
    const data = expenseDoc.data();
    // Create a unique key for the expense
    const key = `${data.date}_${data.amount}_${data.note}_${data.categoryId}`;
    
    if (seen.has(key)) {
      await deleteDoc(doc(db, `users/${user.uid}/expenses`, expenseDoc.id));
      deleted++;
    } else {
      seen.add(key);
    }
  }

  console.log(`Removed ${deleted} duplicate expenses for user ${user.uid}.`);
}

main().catch(console.error);
