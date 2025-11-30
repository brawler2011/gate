import { getSession } from "@/lib/auth";
import { Header } from "./Header";

export async function HeaderWithSession() {
  const session = await getSession();
  console.log(session);
  
  return <Header session={session} />;
}

