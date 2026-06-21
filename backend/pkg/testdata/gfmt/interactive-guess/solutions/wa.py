import sys

def main():
    line = sys.stdin.readline()
    if not line:
        return
    n = int(line.strip())
    print("! 1", flush=True)

if __name__ == '__main__':
    main()
