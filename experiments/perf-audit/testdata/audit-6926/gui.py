from pathlib import Path
import os,subprocess as s,time,statistics as st,math,sys
from collections import Counter
b=Path(__file__).resolve().parent;assert os.environ["DISPLAY"].startswith(":") and not os.environ.get("WAYLAND_DISPLAY");ps=[];warm=[]
for i in range(11):
 root=Path(os.environ["XDG_RUNTIME_DIR"]).parent/("run"+str(i));root.mkdir();e=dict(os.environ,HOME=str(root),XDG_CACHE_HOME=str(root/"cache"),OBIT_HOME=str(root/"data"),OBIT_CONFIG_DIR=str(root/"config"),OBIT_USER_HOME=str(root),XDG_CONFIG_HOME=str(root/"config"));(root/"config").mkdir();(root/"config"/"config.toml").write_text("[update]\ncheck=false\n");t=time.perf_counter();p=s.Popen([str(b/"obit-gui-linux")],env=e,start_new_session=True)
 try:
  w=s.check_output(["timeout","10","xdotool","search","--sync","--onlyvisible","--name","^Obituary$"],env=e,text=True).splitlines()[0];warm.append(1000*(time.perf_counter()-t));g=s.check_output(["xdotool","getwindowgeometry","--shell",w],text=True);d=dict(line.split("=",1) for line in g.splitlines());x,y,W,H=[int(d[k]) for k in ["X","Y","WIDTH","HEIGHT"]];
  while time.perf_counter()-t<10:
   raw=s.check_output(["import","-window",w,"-crop",f"{3*W//4}x{H//2}+{W//8}+{H//4}","-resize","180x90!","-depth","8","gray:-"]);h=Counter(raw);q=len(raw);s1=st.pstdev(raw)
   if s1>8 and sum(n for k,n in h.items() if k<180)>q*.005 and sum(n for k,n in h.items() if k>195)>q*.1:break
   time.sleep(.018)
  else:raise Exception("no painted middle: "+str((s1,min(raw),max(raw))))
  ps.append(1000*(time.perf_counter()-t));s.run(["import","-window",w,str(b/("gui-%d.png"%i))],check=True);print(i,warm[-1],ps[-1],s1,flush=True)
 finally:p.terminate();p.wait(timeout=5)
print("mapped",st.median(warm),max(warm),"painted",st.median(ps),max(ps))
