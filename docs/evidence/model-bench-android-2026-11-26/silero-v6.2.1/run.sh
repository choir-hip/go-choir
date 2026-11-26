#!/system/bin/sh
D=$1; TAG=$2; shift; shift
cd "$D"
export LD_PRELOAD="$D/libhashshim.so" LD_LIBRARY_PATH="$D"
t0=$(date +%s.%N)
"$@" > "$TAG.out" 2> "$TAG.err" &
pid=$!
echo $pid > "$D/pid"
peak=0
while kill -0 $pid 2>/dev/null; do
  while read k v u; do
    if [ "$k" = "VmHWM:" ]; then peak=$v; fi
  done < /proc/$pid/status 2>/dev/null
  sleep 0.05
done
wait $pid
rc=$?
t1=$(date +%s.%N)
echo "exit=$rc wall_start=$t0 wall_end=$t1 vmhwm_kb=$peak" > "$TAG.meta"
