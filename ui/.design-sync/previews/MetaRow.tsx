import { MetaRow, Mono } from "ui";

export const PersonFacts = () => (
  <MetaRow>
    {["kabir.mehta_ug24@example.edu", "Lab assistant", "Makerspace staff", <Mono key="id">312874519023645698</Mono>]}
  </MetaRow>
);

/** Empty facts are dropped, so no dot hangs on the end. */
export const SkipsEmpty = () => (
  <MetaRow>{["meera.pillai@example.edu", null, "Electronics bench", null]}</MetaRow>
);

export const TwoItems = () => <MetaRow>{["12 roles", "Synced 4 min ago"]}</MetaRow>;
