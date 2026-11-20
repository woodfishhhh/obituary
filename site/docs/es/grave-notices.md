# Avisos del cementerio

Para que sirva de algo enterrar un archivo en vez de eliminarlo, hay que poder encontrarlo después. Si una herramienta del agente accede a una ruta enterrada que aún se puede resucitar, o a un elemento dentro de un directorio enterrado, Obituary le indica:

- el número de tumba;
- cuándo se enterró el elemento;
- cuánto tiempo queda para resucitarlo;
- `obit resurrect <id>`.

## Solo informa

- No bloquea ni modifica la llamada a la herramienta.
- Cada tumba se menciona una vez por sesión.
- Claude Code y Codex reciben el aviso antes de ejecutar la herramienta. Para los demás agentes, `obit init --show` indica en qué momento se envía.
- El resto del tiempo no interrumpe: al empezar, el agente solo necesita saber que Obituary está disponible, sin leer ninguna lista.

Si ya tienes Obituary instalado, vuelve a ejecutar `obit init` para recibir los avisos del cementerio.

## Resumen de la sesión (opcional)

Para ahorrar tokens, al inicio de una sesión solo se muestran las propuestas que requieren tu decisión y los elementos que se acaban de registrar automáticamente. Para añadir el resumen, activa «Session summary» en los ajustes de la ventana o configura lo siguiente:

```toml
[hooks]
session_summary = true
```

El agente también verá los elementos de este proyecto que caducan en las próximas 24 horas y los que se han enterrado recientemente y aún se pueden resucitar. Se excluyen los directorios temporales de `obit mktemp`.

## Consultar el cementerio

```powershell
obit graveyard           # what is in the graveyard and when it will be purged
obit graveyard --here    # this folder only
obit resurrect <id>      # put it back; if the name is taken, it comes back renamed, never overwriting
obit resurrect --last    # undo the last deletion: everything one obit bury / obit rm buried comes back
```
