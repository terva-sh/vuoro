These are normalized forge.Change projections of the existing scrubbed,
recorded public GitLab responses in forge/gitlab/testdata/mr3958.json and
mr3991.json. The GitLab adapter's convert function produced them on 2026-10-02;
only the forge kind and stable Ref were supplied. No network call or credential
was used to generate them. Descriptions and non-system note bodies remain the
existing placeholders. CanMerge is unknown because those recordings do not
contain authenticated-person permissions.

The direct fixture tests exercise the recorded reviewer push/reply sequence and
merged state. Table cases start from that same model and replace event lists
with explicit synthetic variants for rules and edge cases absent from the
recordings. Production turn code and tests import no adapter or network code.
