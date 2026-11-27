# Usage: javac-jar <jar> <main class> <source>...; a jar keeps every class javac emits, nested ones included.
# Library jars in the working directory go on the classpath and into the jar, which then runs alone.
set -eu
out=$1 main=$2
shift 2
mkdir classes
cp=.
for lib in *.jar; do
  [ -f "$lib" ] || continue
  cp=$cp:$lib
  (cd classes && jar --extract --file "../$lib")
done
# Their manifests, signatures and module descriptors would conflict with the program's jar; services stay.
rm -f classes/META-INF/MANIFEST.MF classes/META-INF/*.SF classes/META-INF/*.RSA classes/META-INF/*.DSA classes/META-INF/*.EC classes/module-info.class
javac -cp "$cp" -d classes "$@"
jar --create --file "$out" --main-class "$main" -C classes .
