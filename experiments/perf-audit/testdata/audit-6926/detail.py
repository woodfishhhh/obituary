from extra import *
def paths():
 root=B.parents[2];main=root/"cmd/obit/main.go";src=main.read_text();key="root.AddCommand(";a=src.index(key);fx="if len(os.Args)>2 && os.Args[1]==\"hook\" { root.AddCommand(hookcmd.NewCmd()); if err:=root.Execute(); err!=nil {os.Exit(1)};return }\n\t";mod=B/"entry.go";mod.write_text(src[:a]+fx+src[a:]);mapping=json.loads((B/"overlay.json").read_text())["Replace"];mapping[str(main)]=str(mod);rootcache=sp.check_output(["go","env","GOCACHE"],text=True).strip();cache=B/"cache";layer=dict(os.environ,GOCACHE=str(cache),GOCACHEPROG=sp.list2cmdline([str(B/"cachehost.exe"),"obit-test-gocacheprog",str(cache),rootcache]));(B/"route.json").write_text(json.dumps({"Replace":mapping}));dest=B/"routed";dest.mkdir(exist_ok=True);sp.run(["go","build","-overlay="+str(B/"route.json"),"-ldflags=-s -w","-o",str(dest/"obit.exe"),"./cmd/obit"],env=layer,cwd=root,check=True)
 e=env("cost");p=payload("claude","PreToolUse/ordinary","s");srcs=["obit","routed/obit","empty"]
 for nm in ["fast","lazy"]:
  (B/nm).mkdir(exist_ok=True);shutil.copy2(B/(nm+".exe"),B/nm/"obit.exe");srcs.append(nm+"/obit")
 results={k:[] for k in srcs};
 for i in range(31):
  for nm in srcs[i%5:]+srcs[:i%5]:results[nm].append(run(nm,["hook","claude"],e,p)[0])
 for k,v in results.items():show(k,v)
 for nm in srcs:
  if nm!="empty":assert b"deny" in run(nm,["hook","claude"],e,payload("claude","PreToolUse/delete","s"))[1]
def loader():
 e=dict(env("cost"),OBIT_DISABLED="1");v=list(zip(*[list(map(float,run("load",[str(B/"obit.exe")],e)[1].split())) for i in range(21)]));
 for k,a in zip(["CreateProcess-debug","loader-bracket","last-dll-event","early-dll-count","late-dll-count"],v):show(k,a)
def inner():
 for n in [1000,10000]:
  e=env("big"+str(n));_,out,_=run("cli-inner",[],e);groups={}
  for l in out.splitlines():k,x=l.split();groups.setdefault(k.decode(),[]).append(float(x))
  for k,v in groups.items():show(str(n)+"/"+k,v)
if __name__=="__main__":globals()[sys.argv[1]]()
