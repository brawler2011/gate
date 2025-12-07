#!/bin/bash

# ============================================
# Server & Docker Monitor with Telegram Alerts
# ============================================
# Возможности:
#   ✅ Алерт если контейнер жрёт CPU > порога
#   ✅ Алерт если контейнер упал
#   ✅ Алерт при старте сервера (рестарт)
#   ✅ Алерт если диск заполнен
#   ✅ Алерт если глобальный CPU > 90% более 10 минут
#   ✅ Антиспам (cooldown)
# ============================================

# ==================== НАСТРОЙКИ ====================
TELEGRAM_TOKEN="ВСТАВЬ_ТОКЕН"
CHAT_ID="ВСТАВЬ_CHAT_ID"              # Личка или группа (-100xxxx для группы)

CPU_THRESHOLD=80                       # CPU порог для контейнеров (%)
MEMORY_THRESHOLD=90                    # Память порог (%)
DISK_THRESHOLD=85                      # Диск порог (%)

GLOBAL_CPU_THRESHOLD=90                # Глобальный CPU порог (%)
GLOBAL_CPU_DURATION=10                 # Минут подряд для алерта

COOLDOWN_MINUTES=15                    # Антиспам: минут между алертами
STATE_DIR="/var/lib/server-monitor"   # Папка для хранения состояния
LOG_FILE="/var/log/server-monitor.log"

# Контейнеры которые ДОЛЖНЫ работать (если упадут — алерт)
REQUIRED_CONTAINERS="postgres redis gate149-frontend nginx"

# ==================== ФУНКЦИИ ====================

# Создаём папку для состояния
mkdir -p "$STATE_DIR"

# Логирование
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

# Отправка в Telegram
send_telegram() {
    local message="$1"
    local result=$(curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_TOKEN}/sendMessage" \
        -d chat_id="${CHAT_ID}" \
        -d text="${message}" \
        -d parse_mode="HTML" 2>&1)
    
    if echo "$result" | grep -q '"ok":true'; then
        log "Telegram: sent OK"
    else
        log "Telegram: FAILED - $result"
    fi
}

# Проверка cooldown
check_cooldown() {
    local key="$1"
    local marker="${STATE_DIR}/cooldown_${key}"
    
    if [ -f "$marker" ]; then
        local last=$(cat "$marker")
        local now=$(date +%s)
        local diff=$(( (now - last) / 60 ))
        
        if [ "$diff" -lt "$COOLDOWN_MINUTES" ]; then
            return 1  # В cooldown — не слать
        fi
    fi
    
    date +%s > "$marker"
    return 0  # Можно слать
}

# ==================== ПРОВЕРКА 1: СТАРТ СЕРВЕРА ====================
check_server_start() {
    local boot_marker="${STATE_DIR}/last_boot"
    local current_boot=$(who -b | awk '{print $3, $4}')
    
    if [ -f "$boot_marker" ]; then
        local last_boot=$(cat "$boot_marker")
        if [ "$current_boot" != "$last_boot" ]; then
            # Сервер перезагрузился!
            echo "$current_boot" > "$boot_marker"
            
            local uptime=$(uptime -p)
            send_telegram "🔄 <b>SERVER RESTARTED!</b>

🖥 Server: <code>$(hostname)</code>
⏰ Boot time: <code>${current_boot}</code>
⬆️ Uptime: ${uptime}

Checking services..."
            
            log "Server restart detected"
            sleep 5  # Даём контейнерам время подняться
        fi
    else
        # Первый запуск скрипта
        echo "$current_boot" > "$boot_marker"
        log "First run, boot marker created"
    fi
}

# ==================== ПРОВЕРКА 2: УПАВШИЕ КОНТЕЙНЕРЫ ====================
check_container_status() {
    local running_containers=$(docker ps --format "{{.Names}}" 2>/dev/null)
    
    for container in $REQUIRED_CONTAINERS; do
        local status_marker="${STATE_DIR}/container_${container}"
        local is_running=$(echo "$running_containers" | grep -w "$container")
        
        if [ -z "$is_running" ]; then
            # Контейнер НЕ запущен
            if [ ! -f "$status_marker" ] || [ "$(cat $status_marker)" = "running" ]; then
                # Был запущен, теперь упал — алерт!
                echo "down" > "$status_marker"
                
                if check_cooldown "down_${container}"; then
                    send_telegram "🔴 <b>CONTAINER DOWN!</b>

🐳 Container: <code>${container}</code>
🖥 Server: <code>$(hostname)</code>
🕐 Time: $(date '+%Y-%m-%d %H:%M:%S')

Check: <code>docker logs ${container} --tail 100</code>
Restart: <code>docker start ${container}</code>"
                    
                    log "ALERT: Container $container is DOWN"
                fi
            fi
        else
            # Контейнер запущен
            if [ -f "$status_marker" ] && [ "$(cat $status_marker)" = "down" ]; then
                # Был упавшим, теперь поднялся — уведомление
                echo "running" > "$status_marker"
                
                send_telegram "🟢 <b>Container recovered!</b>

🐳 Container: <code>${container}</code>
✅ Status: Running
🕐 Time: $(date '+%Y-%m-%d %H:%M:%S')"
                
                log "Container $container recovered"
            else
                echo "running" > "$status_marker"
            fi
        fi
    done
}

# ==================== ПРОВЕРКА 3: CPU КОНТЕЙНЕРОВ ====================
check_container_cpu() {
    docker stats --no-stream --format "{{.Name}}|{{.CPUPerc}}|{{.MemPerc}}" 2>/dev/null | while IFS='|' read -r name cpu mem; do
        # Парсим CPU
        cpu_clean=$(echo "$cpu" | tr -d '% ')
        cpu_int=${cpu_clean%.*}
        
        # Парсим Memory
        mem_clean=$(echo "$mem" | tr -d '% ')
        mem_int=${mem_clean%.*}
        
        # Проверка CPU
        if [ -n "$cpu_int" ] && [ "$cpu_int" -gt "$CPU_THRESHOLD" ]; then
            if check_cooldown "cpu_${name}"; then
                send_telegram "🔥 <b>HIGH CPU!</b>

🐳 Container: <code>${name}</code>
📊 CPU: <b>${cpu}</b>
💾 Memory: ${mem}
🖥 Server: $(hostname)
🕐 Time: $(date '+%Y-%m-%d %H:%M:%S')

Check: <code>docker logs ${name} --tail 50</code>"
                
                log "ALERT: $name CPU at $cpu"
            fi
        fi
        
        # Проверка Memory
        if [ -n "$mem_int" ] && [ "$mem_int" -gt "$MEMORY_THRESHOLD" ]; then
            if check_cooldown "mem_${name}"; then
                send_telegram "💾 <b>HIGH MEMORY!</b>

🐳 Container: <code>${name}</code>
📊 CPU: ${cpu}
💾 Memory: <b>${mem}</b>
🖥 Server: $(hostname)
🕐 Time: $(date '+%Y-%m-%d %H:%M:%S')"
                
                log "ALERT: $name Memory at $mem"
            fi
        fi
    done
}

# ==================== ПРОВЕРКА 4: ДИСК ====================
check_disk_usage() {
    local disk_usage=$(df / | tail -1 | awk '{print $5}' | tr -d '%')
    
    if [ "$disk_usage" -gt "$DISK_THRESHOLD" ]; then
        if check_cooldown "disk_root"; then
            local disk_info=$(df -h / | tail -1 | awk '{print "Used: "$3" / "$2" ("$5")"}')
            
            send_telegram "💿 <b>DISK ALMOST FULL!</b>

🖥 Server: <code>$(hostname)</code>
📁 Disk: <b>${disk_usage}%</b>
📊 ${disk_info}
🕐 Time: $(date '+%Y-%m-%d %H:%M:%S')

Clean: <code>docker system prune -a</code>"
            
            log "ALERT: Disk at ${disk_usage}%"
        fi
    fi
}

# ==================== ПРОВЕРКА 5: ГЛОБАЛЬНЫЙ CPU ====================
check_global_cpu() {
    local history_file="${STATE_DIR}/global_cpu_history"
    local now=$(date +%s)
    local cpu_idle=$(top -bn1 | grep "Cpu(s)" | awk '{print $8}' | cut -d'.' -f1)
    local cpu_usage=$((100 - cpu_idle))
    
    # Записываем текущее значение: timestamp:cpu
    echo "${now}:${cpu_usage}" >> "$history_file"
    
    # Удаляем записи старше GLOBAL_CPU_DURATION минут
    local cutoff=$((now - GLOBAL_CPU_DURATION * 60))
    local temp_file="${history_file}.tmp"
    
    while IFS=':' read -r timestamp value; do
        if [ "$timestamp" -ge "$cutoff" ] 2>/dev/null; then
            echo "${timestamp}:${value}"
        fi
    done < "$history_file" > "$temp_file"
    mv "$temp_file" "$history_file"
    
    # Проверяем сколько записей и все ли выше порога
    local total_records=$(wc -l < "$history_file")
    local high_records=$(awk -F':' -v threshold="$GLOBAL_CPU_THRESHOLD" '$2 >= threshold' "$history_file" | wc -l)
    
    # Нужно минимум GLOBAL_CPU_DURATION записей (если скрипт раз в минуту)
    local min_records=$GLOBAL_CPU_DURATION
    
    if [ "$total_records" -ge "$min_records" ] && [ "$high_records" -eq "$total_records" ]; then
        # Все записи за последние N минут выше порога!
        if check_cooldown "global_cpu"; then
            local avg_cpu=$(awk -F':' '{sum+=$2; count++} END {print int(sum/count)}' "$history_file")
            local top_processes=$(ps aux --sort=-%cpu | head -6 | tail -5 | awk '{printf "• %s (%.1f%%)\n", $11, $3}')
            
            send_telegram "🔥🔥 <b>SUSTAINED HIGH CPU!</b>

🖥 Server: <code>$(hostname)</code>
📊 CPU: <b>${avg_cpu}%</b> avg over ${GLOBAL_CPU_DURATION} min
⏰ All ${total_records} checks were above ${GLOBAL_CPU_THRESHOLD}%
🕐 Time: $(date '+%Y-%m-%d %H:%M:%S')

<b>Top processes:</b>
<code>${top_processes}</code>

Check: <code>htop</code>"
            
            log "ALERT: Global CPU sustained at ${avg_cpu}% for ${GLOBAL_CPU_DURATION}+ minutes"
        fi
    fi
    
    log "Global CPU: ${cpu_usage}% (${high_records}/${total_records} above threshold)"
}

# ==================== ГЛАВНЫЙ ЗАПУСК ====================
log "=== Monitor check started ==="

check_server_start
check_container_status
check_container_cpu
check_disk_usage
check_global_cpu

log "=== Monitor check finished ==="