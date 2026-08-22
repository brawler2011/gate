import sys

def main():
    line = sys.stdin.read().strip()
    if not line:
        return
    n = int(line)
    if n % 2 == 0:
        print("Even")
    else:
        print("Odd")

if __name__ == '__main__':
    main()
