# Usage: javac-jar <jar> <main class> <source>...; a jar keeps every class javac emits, nested ones included.
set -eu
out=$1 main=$2
shift 2
javac -d classes "$@"
jar --create --file "$out" --main-class "$main" -C classes .
