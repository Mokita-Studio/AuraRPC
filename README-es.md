# AuraRPC

[ 🇺🇸 English ](README.md) | [ 🇪🇸 Español ](README-es.md)

Una app de Discord Rich Presence diminuta, privada y que no te estorba.

AuraRPC te deja poner tu propio estado personalizado en Discord — esa tarjeta de
"Jugando a …" en tu perfil — sin necesitar un juego que lo soporte. Habla
directamente con tu cliente de Discord en tu propia máquina, así que no hay
servidores, ni inicios de sesión, ni tokens de por medio.

Hecho por [Mokita Studio](https://github.com/Mokita-Studio). Gratis y de código abierto.

> ¿Te interesa cómo funciona por dentro? La documentación técnica (en inglés)
> está en [`docs/en/`](docs/en/INDEX.md).

---

## ¿Por qué otra más?

La mayoría de las herramientas de Rich Presence son pesadas, recargadas, o
ambas. AuraRPC hace lo contrario:

- **Ligera.** Un único binario de ~12 MB sin nada que instalar aparte. Vive
  tranquilo en tu bandeja y apenas toca tu RAM.
- **Privada.** Nunca sale a internet — solo abre el pipe local de Discord. Sin
  telemetría, sin cuentas, sin tokens. Jamás se lee tu sesión de Discord.
- **Sencilla.** Rellenas un par de campos, pulsas Conectar y listo. Guarda
  tantos presets como quieras y cambia entre ellos desde la propia bandeja.

---

## Funciones

- Un editor limpio para cada campo de la presencia de Discord: detalles, estado,
  imágenes grande y pequeña, botones, tiempos, tamaño de grupo y tipo de actividad.
- **Presets** — guarda distintos estados y cambia con un solo clic desde la
  bandeja, sin siquiera abrir la ventana.
- **Temas claro y oscuro** que siguen tu barra de tareas de Windows.
- **Funciona en segundo plano** — cierra la ventana y tu estado sigue vivo desde
  la bandeja.
- **Comprobación opcional de actualizaciones** que solo te avisa cuando hay una
  versión nueva. Nunca instala nada a tus espaldas.

---

## Cómo empezar

1. Descarga `AuraRPC.exe` (o el instalador) desde la página de [Releases](../../releases).
2. Entra al [Discord Developer Portal](https://discord.com/developers/applications),
   crea una aplicación y copia su **Application ID**. El nombre de esa app es lo
   que aparecerá como "Jugando a …" en tu perfil.
3. *(Opcional)* En *Rich Presence → Art Assets*, sube las imágenes que quieras
   usar y anota sus **asset keys**.
4. Abre AuraRPC, pega el Application ID, rellena tus datos y pulsa **Conectar**.
5. Pulsa **Guardar** para conservar el preset en tu barra lateral.

Cerrar la ventana no cierra la app — sigue funcionando en la bandeja. Haz clic
derecho en el icono para cambiar de preset o desconectar.

---

## Preguntas frecuentes

**¿Necesito un bot o un token?**
No. AuraRPC solo necesita un Application ID, que es público. Nunca inicia sesión
en tu cuenta ni te pide ningún token.

**¿Por qué necesito un Application ID?**
Discord asocia cada Rich Presence a una app. El nombre de esa app es lo que
aparece como "Jugando a &lt;nombre&gt;" en tu perfil, y sus imágenes subidas son las
que tu estado puede mostrar. Crear una es gratis y toma cosa de un minuto en el
Developer Portal.

**No aparece mi estado — ¿qué reviso?**
Asegúrate de que Discord esté abierto, que el Application ID sea correcto y que
pulsaste Conectar. Discord puede tardar unos segundos en mostrar una presencia
nueva. Comprueba también que *Privacidad de actividad → Mostrar la actividad
actual como mensaje de estado* esté activado en los ajustes de Discord.

**No se ven mis imágenes.**
Las imágenes se referencian por su **asset key** — el nombre que les diste en
*Rich Presence → Art Assets* del Developer Portal. Verifica que la clave coincida
exactamente; los assets recién subidos pueden tardar un rato en estar disponibles.

**Windows me avisa con SmartScreen al abrirla.**
El binario aún no está firmado, así que Windows muestra una advertencia. Pulsa
*Más información → Ejecutar de todas formas*. Siempre puedes leer o compilar el
código tú mismo — es abierto.

**¿Consume muchos recursos?**
No. Es un único binario pequeño que reposa en la bandeja con poca RAM y sin
actividad de red en segundo plano.

**¿Puedo tener más de un estado?**
Sí — guarda tantos presets como quieras y cambia entre ellos al instante desde
el menú de la bandeja.

**¿Funciona en Linux o macOS?**
Por ahora solo Windows. Una versión para Linux está en camino.

---

## Especificaciones técnicas

|                          |                                                  |
| ------------------------ | ------------------------------------------------ |
| Lenguaje                 | Go 1.23+                                          |
| UI                       | [Gio](https://gioui.org) (render por GPU)         |
| Tamaño del binario       | ~12 MB                                            |
| RAM                      | unos MB en reposo en la bandeja; más con la ventana abierta (varía según el sistema) |
| Dependencias en runtime  | ninguna                                           |
| Plataformas (v1)         | Windows 10 1809+ / Windows 11                     |
| IPC con Discord          | named pipe local (`\\.\pipe\discord-ipc-{0..9}`)  |
| Telemetría / red         | ninguna                                           |
| Idiomas                  | Inglés, Español                                   |
| Almacenamiento           | JSON plano en `%APPDATA%\AuraRPC\`                |

---

## Compilar desde el código

Requisitos: **Go 1.23+** y **PowerShell 5.1+**. Opcionalmente **Inno Setup 6**
(para el instalador) y **MinGW-w64 / gcc** (para tests con la flag `-race`).

```powershell
.\scripts\build.ps1      # compila AuraRPC.exe en la raíz del proyecto
.\scripts\package.ps1    # genera el instalador en dist\
```

`build.ps1` regenera el `.syso` embebido automáticamente cuando cambian el icono
o los recursos.

---

## Licencia

MIT — ver [LICENSE](LICENSE). Copyright © 2026 Mokita Studio.
