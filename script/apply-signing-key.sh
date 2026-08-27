#!/usr/bin/env bash
# Replace the signing-key placeholders left by the key-rotation PR with real
# values, once the key exists. Run on the PR branch, review, then commit.
#
#   ./script/apply-signing-key.sh \
#     --primary-fpr           <40 hex> \
#     --primary-expiry        YYYY-MM-DD \
#     --subkey-archivist-fpr  <40 hex> \
#     --subkey-apt-fpr        <40 hex> \
#     [--public-key path/to/public.asc]
#
# Supply whichever subkeys this repository names; the script refuses to finish
# while any placeholder is left unresolved, so a missing argument is an error
# rather than a silently half-applied rotation.
set -euo pipefail

PRIMARY="" EXPIRY="" SUB_ARCHIVIST="" SUB_APT="" PUBKEY="" ROTATED=""

while [ $# -gt 0 ]; do
  case "$1" in
    --primary-fpr)          PRIMARY="${2:?}";       shift 2 ;;
    --primary-expiry)       EXPIRY="${2:?}";        shift 2 ;;
    --subkey-archivist-fpr) SUB_ARCHIVIST="${2:?}"; shift 2 ;;
    --subkey-apt-fpr)       SUB_APT="${2:?}";       shift 2 ;;
    --public-key)           PUBKEY="${2:?}";        shift 2 ;;
    --rotation-date)        ROTATED="${2:?}";       shift 2 ;;
    -h|--help)              sed -n '2,13p' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

die() { printf 'error: %b\n' "$*" >&2; exit 1; }

# A mistyped fingerprint is not something to discover after publishing.
check_fpr() {
  local name="$1" val="$2"
  [ -n "$val" ] || return 0
  [[ "$val" =~ ^[0-9A-Fa-f]{40}$ ]] || die "$name is not 40 hex characters: $val"
}

[ -n "$PRIMARY" ] || die "--primary-fpr is required"
[ -n "$EXPIRY" ]  || die "--primary-expiry is required (YYYY-MM-DD)"
if git grep -q 'REPLACE_ME_ROTATION_DATE' -- . 2>/dev/null && [ -z "$ROTATED" ]; then
  die "this repository has a rotation notice - --rotation-date YYYY-MM-DD is required"
fi
[[ "$EXPIRY" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || die "expiry must be YYYY-MM-DD: $EXPIRY"
check_fpr "--primary-fpr" "$PRIMARY"
check_fpr "--subkey-archivist-fpr" "$SUB_ARCHIVIST"
check_fpr "--subkey-apt-fpr" "$SUB_APT"

PRIMARY="${PRIMARY^^}"; SUB_ARCHIVIST="${SUB_ARCHIVIST^^}"; SUB_APT="${SUB_APT^^}"

# The primary must never be one of the signing subkeys. If it is, the wrong key
# was exported and CI is about to be handed the project's identity.
for s in "$SUB_ARCHIVIST" "$SUB_APT"; do
  [ -z "$s" ] || [ "$s" != "$PRIMARY" ] || die "a subkey fingerprint equals the primary - wrong key exported"
done
[ -z "$SUB_ARCHIVIST" ] || [ -z "$SUB_APT" ] || [ "$SUB_ARCHIVIST" != "$SUB_APT" ] \
  || die "both subkey fingerprints are identical - the point of two subkeys is that they differ"

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

# Spaced form, as fingerprints are conventionally shown to humans.
spaced() { echo "$1" | sed -E 's/(.{4})/\1 /g; s/ $//'; }

mapfile -t files < <(git grep -l 'REPLACE_ME_' -- . || true)
[ "${#files[@]}" -gt 0 ] || die "no placeholders found - has this already been run?"

# Written as `if` blocks rather than `[ -n x ] && cmd`: under `set -e` a false
# test as the final statement of a loop body aborts the whole script.
for f in "${files[@]}"; do
  sed -i -e "s/REPLACE_ME_PRIMARY_FPR_SPACED/$(spaced "$PRIMARY")/g" \
         -e "s/REPLACE_ME_PRIMARY_FPR/${PRIMARY}/g" \
         -e "s/REPLACE_ME_PRIMARY_EXPIRY/${EXPIRY}/g" "$f"
  if [ -n "$SUB_ARCHIVIST" ]; then
    sed -i -e "s/REPLACE_ME_SUBKEY_ARCHIVIST_FPR/${SUB_ARCHIVIST}/g" "$f"
  fi
  if [ -n "$SUB_APT" ]; then
    sed -i -e "s/REPLACE_ME_SUBKEY_APT_FPR_SPACED/$(spaced "$SUB_APT")/g" \
           -e "s/REPLACE_ME_SUBKEY_APT_FPR/${SUB_APT}/g" "$f"
  fi
  if [ -n "$ROTATED" ]; then
    sed -i -e "s/REPLACE_ME_ROTATION_DATE/${ROTATED}/g" "$f"
  fi
  echo "updated $f"
done

if [ -n "$PUBKEY" ]; then
  [ -f "$PUBKEY" ] || die "public key file not found: $PUBKEY"
  grep -q 'BEGIN PGP PUBLIC KEY BLOCK' "$PUBKEY" || die "not an armoured public key: $PUBKEY"
  # Refuse to publish a key that is not the one named on the command line.
  got="$(gpg --show-keys --with-colons "$PUBKEY" | awk -F: '/^fpr:/{print $10; exit}')"
  [ "$got" = "$PRIMARY" ] || die "public key fingerprint is $got, expected $PRIMARY"
  for dest in apt/guysinc-apt.asc; do
    [ -f "$dest" ] || continue
    cp "$PUBKEY" "$dest"; echo "updated $dest"
  done
fi

if git grep -n 'REPLACE_ME_' -- . ; then
  die "placeholders above are still unresolved. Run 'git checkout -- .' to undo this\n       partial run, then retry with the missing --subkey-* argument."
fi
printf '\nDone. Review with '\''git diff'\'', then commit.\n'
