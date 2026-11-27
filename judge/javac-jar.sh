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
rm -rf classes/META-INF
javac -cp "$cp" -d classes "$@"
jar --create --file "$out" --main-class "$main" -C classes .
