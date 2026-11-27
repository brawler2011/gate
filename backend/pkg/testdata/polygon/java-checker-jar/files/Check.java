import java.nio.file.Files;
import java.nio.file.Paths;
import tokens.Tokens;

// The checker compiles and runs against files/tokens.jar, as NERC checkers do against testlib4j.jar.
public class Check {
    public static void main(String[] args) throws Exception {
        String output = new String(Files.readAllBytes(Paths.get(args[1])));
        String answer = new String(Files.readAllBytes(Paths.get(args[2])));
        System.exit(Tokens.same(output, answer) ? 0 : 1);
    }
}
