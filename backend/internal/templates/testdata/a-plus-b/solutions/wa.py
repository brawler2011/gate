import sys

def main():
    line = sys.stdin.read()
    if not line.strip():
        return
    parts = line.split()
    if len(parts) >= 2:
        a = int(parts[0])
        b = int(parts[1])
        print(a - b)

if __name__ == '__main__':
    main()
