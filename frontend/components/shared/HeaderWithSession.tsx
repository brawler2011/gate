import { getCurrentUser } from "@/lib/auth";
import { Header } from "./Header";

export async function HeaderWithSession() {
  const user = await getCurrentUser();
  
  return <Header user={user} />;
}
