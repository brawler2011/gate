import sys

def main():
    line = sys.stdin.readline()
    if not line:
        return
    n = int(line.strip())
    
    low, high = 1, n
    while low <= high:
        mid = (low + high) // 2
        print(f"? {mid}", flush=True)
        
        response = sys.stdin.readline()
        if not response:
            break
        response = response.strip()
        
        if response == "=":
            print(f"! {mid}", flush=True)
            break
        elif response == ">":
            low = mid + 1
        else:
            high = mid - 1

if __name__ == '__main__':
    main()
