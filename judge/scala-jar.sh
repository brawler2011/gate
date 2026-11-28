# Usage: scala-jar <jar> <source>...; the jar carries the Scala library, so plain java runs it.
set -eu
out=$1
shift
scalac -d "$out" "$@"
mkdir libs
for lib in "$SCALA_LIB"/scala-library-*.jar "$SCALA_LIB"/scala3-library_3-*.jar; do
  (cd libs && jar --extract --file "$lib")
done
rm -f libs/META-INF/MANIFEST.MF
jar --update --file "$out" -C libs .
